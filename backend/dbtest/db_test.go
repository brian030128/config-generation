package dbtest

import (
	"context"
	"testing"
)

var expectedTables = []string{
	"users",
	"environments",
	"projects",
	"project_config_templates",
	"project_config_values",
	"global_values",
	"deployments",
	"deployment_entries",
	"deployment_entry_global_refs",
	"pull_requests",
	"pr_changes",
	"pr_approvals",
	"roles",
	"role_permissions",
	"user_roles",
}

func TestMigrationsCreateAllTables(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := StartPostgres(ctx)
	if err != nil {
		t.Fatalf("StartPostgres: %v", err)
	}
	defer cleanup()
	defer db.Close()

	for _, table := range expectedTables {
		var exists bool
		err := db.QueryRowContext(ctx,
			`SELECT EXISTS (
				SELECT 1 FROM information_schema.tables
				WHERE table_schema = 'public' AND table_name = $1
			)`, table).Scan(&exists)
		if err != nil {
			t.Fatalf("query table %s: %v", table, err)
		}
		if !exists {
			t.Errorf("expected table %q to exist", table)
		}
	}
}

func TestBasicCRUD(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := StartPostgres(ctx)
	if err != nil {
		t.Fatalf("StartPostgres: %v", err)
	}
	defer cleanup()
	defer db.Close()

	// Insert a user
	var userID int64
	err = db.QueryRowContext(ctx,
		`INSERT INTO users (username, display_name) VALUES ($1, $2) RETURNING id`,
		"alice", "Alice Smith",
	).Scan(&userID)
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}
	if userID == 0 {
		t.Fatal("expected non-zero user ID")
	}

	// Read back
	var username, displayName string
	err = db.QueryRowContext(ctx,
		`SELECT username, display_name FROM users WHERE id = $1`, userID,
	).Scan(&username, &displayName)
	if err != nil {
		t.Fatalf("select user: %v", err)
	}
	if username != "alice" || displayName != "Alice Smith" {
		t.Errorf("got (%q, %q), want (alice, Alice Smith)", username, displayName)
	}

	// Insert a project referencing the user (FK test)
	var projectID int64
	err = db.QueryRowContext(ctx,
		`INSERT INTO projects (name, created_by) VALUES ($1, $2) RETURNING id`,
		"my-project", userID,
	).Scan(&projectID)
	if err != nil {
		t.Fatalf("insert project: %v", err)
	}
	if projectID == 0 {
		t.Fatal("expected non-zero project ID")
	}
}

func TestForeignKeyConstraint(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := StartPostgres(ctx)
	if err != nil {
		t.Fatalf("StartPostgres: %v", err)
	}
	defer cleanup()
	defer db.Close()

	// Inserting a project with a non-existent user should fail
	_, err = db.ExecContext(ctx,
		`INSERT INTO projects (name, created_by) VALUES ($1, $2)`,
		"bad-project", 99999,
	)
	if err == nil {
		t.Fatal("expected FK violation error, got nil")
	}
}
