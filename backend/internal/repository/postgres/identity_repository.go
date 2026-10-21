package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"catbrasil/backend/internal/domain"
	"catbrasil/backend/pkg/identity"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type IdentityRepository struct {
	pool   *pgxpool.Pool
	cipher *identity.Cipher
}

func NewIdentityRepository(pool *pgxpool.Pool, secret string) (*IdentityRepository, error) {
	cipher, err := identity.NewCipher(secret)
	if err != nil {
		return nil, err
	}
	return &IdentityRepository{pool: pool, cipher: cipher}, nil
}

func (r *IdentityRepository) prepare(person domain.IdentityPerson) ([]byte, string, error) {
	normalized, err := identity.NormalizePerson(identity.Person(person))
	if err != nil {
		return nil, "", err
	}
	data, err := r.cipher.Encrypt(normalized)
	if err != nil {
		return nil, "", err
	}
	return data, r.cipher.Digest(normalized.CPF), nil
}

func identityConstraintError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		if pgErr.ConstraintName == "user_identities_cpf_digest_key" {
			return domain.ErrIdentityAlreadyExists
		}
		if pgErr.ConstraintName == "user_identities_pkey" {
			return domain.ErrIdentityAlreadySubmitted
		}
	}
	return err
}

// CreateUserWithIdentity grava conta e documento na mesma transação.
func (r *IdentityRepository) CreateUserWithIdentity(ctx context.Context, user *domain.User, person domain.IdentityPerson) error {
	data, digest, err := r.prepare(person)
	if err != nil {
		return err
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := createUser(ctx, tx, user); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO user_identities(user_id, payload_ciphertext, cpf_digest) VALUES ($1,$2,$3)`, user.ID, data, digest); err != nil {
		return identityConstraintError(err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO identity_events(user_id, actor_id, action, cpf_digest) VALUES ($1,$1,'submitted',$2)`, user.ID, digest); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// Submit aceita contas legadas sem documento e nova submissão após rejeição.
func (r *IdentityRepository) Submit(ctx context.Context, userID string, person domain.IdentityPerson) error {
	data, digest, err := r.prepare(person)
	if err != nil {
		return err
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var status string
	err = tx.QueryRow(ctx, `SELECT status FROM user_identities WHERE user_id=$1 FOR UPDATE`, userID).Scan(&status)
	action := "resubmitted"
	if errors.Is(err, pgx.ErrNoRows) {
		action = "submitted"
		if _, err = tx.Exec(ctx, `INSERT INTO user_identities(user_id, payload_ciphertext, cpf_digest) VALUES ($1,$2,$3)`, userID, data, digest); err != nil {
			return identityConstraintError(err)
		}
	} else if err != nil {
		return err
	} else if status != "rejected" {
		return domain.ErrIdentityAlreadySubmitted
	} else {
		if _, err = tx.Exec(ctx, `UPDATE user_identities SET payload_ciphertext=$2, cpf_digest=$3, status='pending', review_ref=NULL, reviewed_by=NULL, reviewed_at=NULL, updated_at=NOW() WHERE user_id=$1`, userID, data, digest); err != nil {
			return identityConstraintError(err)
		}
	}
	if _, err = tx.Exec(ctx, `INSERT INTO identity_events(user_id, actor_id, action, cpf_digest) VALUES ($1,$1,$2,$3)`, userID, action, digest); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *IdentityRepository) Status(ctx context.Context, userID string) (string, error) {
	var status string
	err := r.pool.QueryRow(ctx, `SELECT status FROM user_identities WHERE user_id=$1`, userID).Scan(&status)
	if errors.Is(err, pgx.ErrNoRows) {
		return "missing", nil
	}
	return status, err
}

func (r *IdentityRepository) VerifiedPerson(ctx context.Context, userID string) (domain.IdentityPerson, error) {
	var data []byte
	var status string
	err := r.pool.QueryRow(ctx, `SELECT payload_ciphertext, status FROM user_identities WHERE user_id=$1`, userID).Scan(&data, &status)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.IdentityPerson{}, domain.ErrIdentityNotVerified
	}
	if err != nil {
		return domain.IdentityPerson{}, err
	}
	if status != "verified" {
		return domain.IdentityPerson{}, domain.ErrIdentityNotVerified
	}
	person, err := r.cipher.Decrypt(data)
	if err != nil {
		return domain.IdentityPerson{}, fmt.Errorf("falha ao ler identidade aprovada: %w", err)
	}
	return domain.IdentityPerson(person), nil
}

// Decide grava a decisão e seu evento imutável no mesmo commit.
func (r *IdentityRepository) Decide(ctx context.Context, userID, actorID, status, reviewRef string) error {
	reviewRef = strings.TrimSpace(reviewRef)
	if (status != "verified" && status != "rejected") || len(reviewRef) < 5 || len(reviewRef) > 200 || strings.TrimSpace(actorID) == "" || strings.ContainsAny(reviewRef, "\r\n\x00") {
		return domain.ErrIdentityState
	}
	if actorID == userID {
		return domain.ErrIdentityState
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var digest string
	err = tx.QueryRow(ctx, `UPDATE user_identities SET status=$2, review_ref=$3, reviewed_by=$4, reviewed_at=NOW(), updated_at=NOW() WHERE user_id=$1 AND status='pending' RETURNING cpf_digest`, userID, status, reviewRef, actorID).Scan(&digest)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrIdentityState
	}
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO identity_events(user_id, actor_id, action, cpf_digest, review_ref) VALUES ($1,$2,$3,$4,$5)`, userID, actorID, status, digest, reviewRef); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// CheckKey fails startup if stored PII cannot be decrypted with the configured key.
func (r *IdentityRepository) CheckKey(ctx context.Context) error {
	var data []byte
	err := r.pool.QueryRow(ctx, `SELECT payload_ciphertext FROM user_identities LIMIT 1`).Scan(&data)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	_, err = r.cipher.Decrypt(data)
	return err
}

func (r *IdentityRepository) Pending(ctx context.Context) ([]string, error) {
	rows, err := r.pool.Query(ctx, `SELECT user_id FROM user_identities WHERE status='pending' ORDER BY created_at ASC LIMIT 200`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := make([]string, 0)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// Inspect registra acesso a PII antes de devolvê-la para conferência humana.
func (r *IdentityRepository) Inspect(ctx context.Context, userID, actorID string) (domain.IdentityPerson, error) {
	if strings.TrimSpace(actorID) == "" {
		return domain.IdentityPerson{}, domain.ErrIdentityState
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.IdentityPerson{}, err
	}
	defer tx.Rollback(ctx)
	var data []byte
	var digest, status string
	err = tx.QueryRow(ctx, `SELECT payload_ciphertext,cpf_digest,status FROM user_identities WHERE user_id=$1 FOR SHARE`, userID).Scan(&data, &digest, &status)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.IdentityPerson{}, domain.ErrIdentityState
	}
	if err != nil {
		return domain.IdentityPerson{}, err
	}
	if status != "pending" {
		return domain.IdentityPerson{}, domain.ErrIdentityState
	}
	person, err := r.cipher.Decrypt(data)
	if err != nil {
		return domain.IdentityPerson{}, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO identity_events(user_id,actor_id,action,cpf_digest) VALUES ($1,$2,'inspected',$3)`, userID, actorID, digest); err != nil {
		return domain.IdentityPerson{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return domain.IdentityPerson{}, err
	}
	return domain.IdentityPerson(person), nil
}
