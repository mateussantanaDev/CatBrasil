package migrations

import "testing"

func TestEmbeddedMigrationsAreOrderedAndChecksummed(t *testing.T) {
	all, err := Available()
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 14 {
		t.Fatalf("esperadas 14 migrações, encontradas %d", len(all))
	}
	for index, migration := range all {
		if migration.Version != index+1 || migration.SQL == "" || len(migration.Checksum) != 64 {
			t.Fatalf("migração inválida: %+v", migration)
		}
	}
	if err := validateHistory(all, map[int]string{1: all[0].Checksum, 3: all[2].Checksum}); err == nil {
		t.Fatal("histórico com lacuna deveria falhar")
	}
	if err := validateHistory(all, map[int]string{1: "alterado"}); err == nil {
		t.Fatal("checksum alterado deveria falhar")
	}
}
