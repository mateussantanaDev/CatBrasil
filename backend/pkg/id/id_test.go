package id_test

import (
	"fmt"
	"strings"
	"sync"
	"testing"

	"catbrasil/backend/pkg/id"
)

func TestPrefixValidation(t *testing.T) {
	validPrefixes := []string{
		"PIX",
		"WDR",
		"tx",
		"tx-lvl",
		"tx-ins",
		"com",
		"mach",
		"mach-bonus",
		"mach-parts",
		"ref",
		"evt",
		"promo",
		"RENT",
		"YIELD",
		"INS",
		"SPIN",
		"ADJ",
		"1",
		"123",
		"a-b-c",
		strings.Repeat("a", 30),
	}

	for _, p := range validPrefixes {
		if err := id.ValidatePrefix(p); err != nil {
			t.Errorf("prefixo válido %q rejeitado: %v", p, err)
		}
		generated, err := id.New(p)
		if err != nil {
			t.Errorf("falha ao gerar ID para prefixo %q: %v", p, err)
		}
		if !id.HasPrefix(generated, p) {
			t.Errorf("ID gerado %q não contém prefixo %q", generated, p)
		}
		parts := strings.Split(generated, "-")
		// O último segmento deve ter exatamente 32 caracteres hexadecimais (16 bytes)
		hexPart := parts[len(parts)-1]
		if len(hexPart) != 32 {
			t.Errorf("tamanho do sufixo hex inválido para %q: esperado 32, obteve %d", generated, len(hexPart))
		}
	}

	invalidPrefixes := []string{
		"",
		" ",
		"a_b",
		"prefix_under",
		"prefix com espaco",
		"prefix/slash",
		"prefix.dot",
		"prefix@at",
		"prefix:colon",
		strings.Repeat("a", 31), // > 30 chars
	}

	for _, p := range invalidPrefixes {
		if err := id.ValidatePrefix(p); err == nil {
			t.Errorf("prefixo inválido %q deveria ter sido rejeitado", p)
		}
		if _, err := id.New(p); err == nil {
			t.Errorf("New(%q) deveria ter falhado com erro", p)
		}
	}
}

func TestConcurrentUniqueness_10000(t *testing.T) {
	const total = 10000
	const concurrency = 50
	const perWorker = total / concurrency

	var set sync.Map
	var wg sync.WaitGroup
	errCh := make(chan error, total)

	prefixes := []string{"tx", "mach", "ref", "WDR", "PIX", "evt", "promo", "com", "tx-lvl", "tx-ins"}

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			prefix := prefixes[workerID%len(prefixes)]
			for j := 0; j < perWorker; j++ {
				generated, err := id.New(prefix)
				if err != nil {
					errCh <- fmt.Errorf("worker %d falhou ao gerar ID: %w", workerID, err)
					return
				}
				if !id.HasPrefix(generated, prefix) {
					errCh <- fmt.Errorf("ID %s não bate com prefixo %s", generated, prefix)
					return
				}
				if _, exists := set.LoadOrStore(generated, true); exists {
					errCh <- fmt.Errorf("colisão detectada sob concorrência para ID: %s", generated)
					return
				}
			}
		}(i)
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		t.Fatal(err)
	}
}
