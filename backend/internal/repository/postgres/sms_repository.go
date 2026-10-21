package postgres

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"

	"catbrasil/backend/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type SMSRepository struct {
	pool   *pgxpool.Pool
	secret []byte
}

func NewSMSRepository(pool *pgxpool.Pool, secret string) *SMSRepository {
	return &SMSRepository{pool: pool, secret: []byte(secret)}
}

func (r *SMSRepository) codeMAC(phone, code string) string {
	mac := hmac.New(sha256.New, r.secret)
	mac.Write([]byte(phone))
	mac.Write([]byte{0})
	mac.Write([]byte(code))
	return hex.EncodeToString(mac.Sum(nil))
}

func (r *SMSRepository) CreateCode(ctx context.Context, phone, code string, expiresAt time.Time) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(712348, hashtext($1))`, phone); err != nil {
		return err
	}
	var inCooldown bool
	err = tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM sms_verifications WHERE phone=$1 AND created_at > CURRENT_TIMESTAMP - interval '1 minute')`, phone).Scan(&inCooldown)
	if err != nil {
		return err
	}
	if inCooldown {
		return domain.ErrSMSCooldownActive
	}
	if _, err = tx.Exec(ctx, `UPDATE sms_verifications SET used=TRUE WHERE phone=$1 AND used=FALSE`, phone); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO sms_verifications (phone, code, expires_at, used, created_at) VALUES ($1,$2,$3,FALSE,CURRENT_TIMESTAMP)`, phone, r.codeMAC(phone, code), expiresAt); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *SMSRepository) InvalidateCode(ctx context.Context, phone, code string) error {
	// A rejeição do gateway não deve manter um desafio válido nem bloquear o reenvio.
	_, err := r.pool.Exec(ctx, `DELETE FROM sms_verifications WHERE phone=$1 AND code=$2 AND used=FALSE`, phone, r.codeMAC(phone, code))
	return err
}

func (r *SMSRepository) VerifyCode(ctx context.Context, phone, code string) (bool, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)
	var id int64
	var stored string
	var attempts int
	err = tx.QueryRow(ctx, `SELECT id,code,attempts FROM sms_verifications WHERE phone=$1 AND used=FALSE AND expires_at>CURRENT_TIMESTAMP ORDER BY created_at DESC LIMIT 1 FOR UPDATE`, phone).Scan(&id, &stored, &attempts)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	valid := attempts < 5 && hmac.Equal([]byte(stored), []byte(r.codeMAC(phone, code)))
	if valid {
		_, err = tx.Exec(ctx, `UPDATE sms_verifications SET used=TRUE WHERE id=$1`, id)
	} else {
		_, err = tx.Exec(ctx, `UPDATE sms_verifications SET attempts=attempts+1, used=(attempts+1>=5) WHERE id=$1`, id)
	}
	if err != nil {
		return false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return false, err
	}
	return valid, nil
}
