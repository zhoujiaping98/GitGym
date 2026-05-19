package main

import "testing"

func TestShouldCheckInitialSchemaBaseline(t *testing.T) {
	t.Parallel()

	if !shouldCheckInitialSchemaBaseline("db/migrations/0001_initial.sql") {
		t.Fatal("expected initial migration to allow baseline detection")
	}

	if shouldCheckInitialSchemaBaseline("db/migrations/0002_seed_practice_catalog.sql") {
		t.Fatal("expected seed migration to execute instead of being baseline-recorded")
	}
}
