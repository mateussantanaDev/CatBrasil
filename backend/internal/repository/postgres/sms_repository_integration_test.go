package postgres_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"catbrasil/backend/internal/domain"
	"catbrasil/backend/internal/repository/postgres"
)

func TestSMSCodeHashedCooldownAndSingleUse(t *testing.T) {
	pool := financialDB(t)
	ctx := context.Background()
	repo := postgres.NewSMSRepository(pool, "sms-test-secret-at-least-32-characters")
	phone := "+5511999999999"
	if err := repo.CreateCode(ctx, phone, "123456", time.Now().Add(10*time.Minute)); err != nil {
		t.Fatal(err)
	}
	var stored string
	if err := pool.QueryRow(ctx, `SELECT code FROM sms_verifications WHERE phone=$1 ORDER BY id DESC LIMIT 1`, phone).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if stored == "123456" || len(stored) != 64 {
		t.Fatalf("código não foi protegido: %q", stored)
	}
	if err := repo.CreateCode(ctx, phone, "654321", time.Now().Add(10*time.Minute)); err != domain.ErrSMSCooldownActive {
		t.Fatalf("cooldown: %v", err)
	}
	var wg sync.WaitGroup
	results := make(chan bool, 20)
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			valid, err := repo.VerifyCode(ctx, phone, "123456")
			if err != nil {
				t.Error(err)
			}
			results <- valid
		}()
	}
	wg.Wait()
	close(results)
	success := 0
	for valid := range results {
		if valid {
			success++
		}
	}
	if success != 1 {
		t.Fatalf("código aceito %d vezes; esperado uma", success)
	}
}

func TestSMSCodeLocksAfterFiveFailures(t *testing.T) {
	pool := financialDB(t)
	ctx := context.Background()
	repo := postgres.NewSMSRepository(pool, "sms-test-secret-at-least-32-characters")
	phone := "+5511988888888"
	if err := repo.CreateCode(ctx, phone, "123456", time.Now().Add(10*time.Minute)); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 5; i++ {
		valid, err := repo.VerifyCode(ctx, phone, "999999")
		if err != nil || valid {
			t.Fatalf("tentativa %d: valid=%v err=%v", i, valid, err)
		}
	}
	valid, err := repo.VerifyCode(ctx, phone, "123456")
	if err != nil || valid {
		t.Fatalf("código bloqueado aceito: valid=%v err=%v", valid, err)
	}
}

func TestExpiredSMSCodeCannotRegister(t *testing.T) {
	pool := financialDB(t)
	ctx := context.Background()
	repo := postgres.NewSMSRepository(pool, "sms-test-secret-at-least-32-characters")
	phone := "+5511966666666"
	if err := repo.CreateCode(ctx, phone, "123456", time.Now().Add(10*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE sms_verifications SET expires_at=CURRENT_TIMESTAMP - interval '1 minute' WHERE phone=$1`, phone); err != nil {
		t.Fatal(err)
	}
	valid, err := repo.VerifyCode(ctx, phone, "123456")
	if err != nil || valid {
		t.Fatalf("código expirado aceito: valid=%v err=%v", valid, err)
	}
}

func TestFailedSMSDeliveryInvalidatesChallengeAndAllowsRetry(t *testing.T) {
	pool := financialDB(t)
	ctx := context.Background()
	repo := postgres.NewSMSRepository(pool, "sms-test-secret-at-least-32-characters")
	phone := "+5511977777777"
	if err := repo.CreateCode(ctx, phone, "123456", time.Now().Add(10*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := repo.InvalidateCode(ctx, phone, "123456"); err != nil {
		t.Fatal(err)
	}
	valid, err := repo.VerifyCode(ctx, phone, "123456")
	if err != nil || valid {
		t.Fatalf("desafio não entregue aceito: valid=%v err=%v", valid, err)
	}
	if err := repo.CreateCode(ctx, phone, "654321", time.Now().Add(10*time.Minute)); err != nil {
		t.Fatalf("reenvio imediato após falha deveria funcionar: %v", err)
	}
}
