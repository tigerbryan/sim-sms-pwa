package auth

import (
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestKalkunPasswordMigration(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("migration-test-password"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	phpHash := "$2y$" + string(hash)[4:]
	if !VerifyPassword("migration-test-password", phpHash) || VerifyPassword("incorrect", phpHash) {
		t.Fatal("migrated PHP password acceptance is incorrect")
	}
}
