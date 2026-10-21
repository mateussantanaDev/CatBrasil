package identity

import "testing"

func TestCPFValidationAndEncryptedIdentity(t *testing.T) {
	for _, invalid := range []string{"", "00000000000", "52998224724", "5299822472x"} {
		if _, err := NormalizeCPF(invalid); err == nil {
			t.Fatalf("CPF inválido aceito: %q", invalid)
		}
	}
	person, err := NormalizePerson(Person{CPF: "529.982.247-25", LegalName: "Pessoa  Exemplo", Email: "TESTE@example.invalid"})
	if err != nil {
		t.Fatal(err)
	}
	if person.CPF != "52998224725" || person.Email != "teste@example.invalid" {
		t.Fatalf("normalização incorreta: %+v", person)
	}
	cipher, err := NewCipher("secret-long-enough-for-identity-tests-123")
	if err != nil {
		t.Fatal(err)
	}
	data, err := cipher.Encrypt(person)
	if err != nil {
		t.Fatal(err)
	}
	copyData := append([]byte(nil), data...)
	copyData[len(copyData)-1] ^= 0x01
	if _, err := cipher.Decrypt(copyData); err == nil {
		t.Fatal("alteração da identidade cifrada aceita")
	}
	got, err := cipher.Decrypt(data)
	if err != nil || got != person {
		t.Fatalf("roundtrip: %+v, %v", got, err)
	}
	other, _ := NewCipher("another-secret-long-enough-for-tests-123")
	if _, err := other.Decrypt(data); err == nil {
		t.Fatal("chave incorreta aceitou identidade")
	}
	if cipher.Digest(person.CPF) == other.Digest(person.CPF) {
		t.Fatal("digest não depende da chave")
	}
}
