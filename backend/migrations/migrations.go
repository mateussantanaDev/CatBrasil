package migrations

import _ "embed"

// Fixtures legadas usadas apenas por testes de integridade da migração inicial.
// O CLI e a API usam Available, Apply e CheckCurrent em runner.go, que embutem
// e validam todas as migrações versionadas *.up.sql.
//go:embed 000001_init_schema.up.sql
var SchemaUpSQL string

//go:embed 000001_init_schema.down.sql
var SchemaDownSQL string
