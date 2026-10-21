package postgres_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"catbrasil/backend/internal/domain"
	"catbrasil/backend/internal/repository/postgres"
)

func TestConcurrentIdentityRegistrationAndReview(t *testing.T) {
	pool := financialDB(t)
	ctx := context.Background()
	repo, err := postgres.NewIdentityRepository(pool, "identity-integration-secret-32-characters")
	if err != nil {
		t.Fatal(err)
	}
	person := domain.IdentityPerson{CPF: "52998224725", LegalName: "Pessoa Teste", Email: "teste@example.invalid"}
	ids := []string{"identity-user-a", "identity-user-b"}
	errs := make([]error, len(ids))
	var wg sync.WaitGroup
	for i, id := range ids {
		wg.Add(1)
		go func(i int, id string) {
			defer wg.Done()
			errs[i] = repo.CreateUserWithIdentity(ctx, &domain.User{ID: id, Username: id, PasswordHash: "hash", Nivel: 1}, person)
		}(i, id)
	}
	wg.Wait()
	succeeded, duplicate := 0, 0
	winner := ""
	for i, err := range errs {
		if err == nil {
			succeeded++
			winner = ids[i]
		} else if errors.Is(err, domain.ErrIdentityAlreadyExists) {
			duplicate++
		} else {
			t.Fatal(err)
		}
	}
	if succeeded != 1 || duplicate != 1 {
		t.Fatalf("cadastros=%d, duplicados=%d", succeeded, duplicate)
	}
	var users int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM users WHERE id=ANY($1)`, ids).Scan(&users); err != nil || users != 1 {
		t.Fatalf("registro órfão: %d, %v", users, err)
	}
	if status, err := repo.Status(ctx, winner); err != nil || status != "pending" {
		t.Fatalf("status inicial: %s, %v", status, err)
	}
	if _, err := repo.VerifiedPerson(ctx, winner); !errors.Is(err, domain.ErrIdentityNotVerified) {
		t.Fatalf("pagador não revisado liberado: %v", err)
	}
	inspected, err := repo.Inspect(ctx, winner, "director-test")
	if err != nil || inspected.CPF != person.CPF {
		t.Fatalf("inspeção: %+v, %v", inspected, err)
	}
	if err := repo.Decide(ctx, winner, winner, "verified", "ticket-self-review"); !errors.Is(err, domain.ErrIdentityState) {
		t.Fatalf("autoaprovação aceita: %v", err)
	}
	if err := repo.Decide(ctx, winner, "director-test", "verified", "ticket-test-identity"); err != nil {
		t.Fatal(err)
	}
	if got, err := repo.VerifiedPerson(ctx, winner); err != nil || got.CPF != person.CPF {
		t.Fatalf("pagador revisado: %+v, %v", got, err)
	}
	if err := repo.Decide(ctx, winner, "director-test", "rejected", "ticket-test-second"); !errors.Is(err, domain.ErrIdentityState) {
		t.Fatalf("revisão duplicada aceita: %v", err)
	}
	var events int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM identity_events WHERE user_id=$1`, winner).Scan(&events); err != nil || events != 3 {
		t.Fatalf("trilha: %d, %v", events, err)
	}
}
