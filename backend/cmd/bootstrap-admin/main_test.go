package main

import "testing"

func TestBootstrapRejectsDemoAndWeakCredentials(t *testing.T) {
	for _, username := range []string{"admin", "catadmin", "gestor_master", "teste", "demo_operator", "seed_001", "a", "bad name"} {
		if validUsername(username) {
			t.Errorf("username reservado aceito: %q", username)
		}
	}
	if !validUsername("diretor_mateus") {
		t.Fatal("username válido rejeitado")
	}
	for _, password := range []string{"short", "diretor_mateus", "123456", " senha-com-espaco-longo ", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "apenasletrasminusculassemnumeros"} {
		if validatePassword(password, "diretor_mateus", "123456") == nil {
			t.Errorf("senha fraca ou inválida aceita")
		}
	}
	if err := validatePassword("senha-aleatoria-2026-de-mais-dezesseis", "diretor_mateus", "123456"); err != nil {
		t.Fatalf("senha longa rejeitada: %v", err)
	}
}
