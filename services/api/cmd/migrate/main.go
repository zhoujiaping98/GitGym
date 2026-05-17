package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	mysqlcfg "github.com/go-sql-driver/mysql"
	_ "github.com/go-sql-driver/mysql"
)

func main() {
	dsn := strings.TrimSpace(os.Getenv("MYSQL_DSN"))
	if dsn == "" {
		log.Fatal("MYSQL_DSN is required")
	}

	cfg, err := mysqlcfg.ParseDSN(dsn)
	if err != nil {
		log.Fatalf("parse MYSQL_DSN: %v", err)
	}
	cfg.MultiStatements = true

	db, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatalf("ping database: %v", err)
	}

	if err := ensureMigrationTable(db); err != nil {
		log.Fatalf("ensure schema_migrations table: %v", err)
	}

	migrationPaths, err := filepath.Glob(filepath.Join("db", "migrations", "*.sql"))
	if err != nil {
		log.Fatalf("list migrations: %v", err)
	}
	if len(migrationPaths) == 0 {
		log.Fatal("no migration files found under db/migrations")
	}
	sort.Strings(migrationPaths)

	for _, migrationPath := range migrationPaths {
		applied, err := migrationApplied(db, migrationPath)
		if err != nil {
			log.Fatalf("check migration state for %s: %v", migrationPath, err)
		}
		if applied {
			fmt.Printf("skipped %s (already applied)\n", migrationPath)
			continue
		}

		existingCount, err := countExistingTables(db, []string{
			"users",
			"auth_accounts",
			"user_sessions",
			"workspace_templates",
			"scenarios",
			"practice_sessions",
			"command_runs",
			"repo_snapshots",
			"session_events",
			"session_resets",
		})
		if err != nil {
			log.Fatalf("inspect existing tables for %s: %v", migrationPath, err)
		}

		if existingCount > 0 {
			if existingCount != 10 {
				log.Fatalf("migration %s is not recorded, but only %d/10 expected tables exist; refusing to continue", migrationPath, existingCount)
			}
			if err := recordMigration(db, migrationPath); err != nil {
				log.Fatalf("record baseline migration %s: %v", migrationPath, err)
			}
			fmt.Printf("recorded %s as already present\n", migrationPath)
			continue
		}

		contents, err := os.ReadFile(migrationPath)
		if err != nil {
			log.Fatalf("read migration %s: %v", migrationPath, err)
		}
		query := strings.TrimSpace(string(contents))
		if query == "" {
			continue
		}
		if _, err := db.Exec(query); err != nil {
			log.Fatalf("apply migration %s: %v", migrationPath, err)
		}
		if err := recordMigration(db, migrationPath); err != nil {
			log.Fatalf("record migration %s: %v", migrationPath, err)
		}
		fmt.Printf("applied %s\n", migrationPath)
	}
}

func ensureMigrationTable(db *sql.DB) error {
	_, err := db.Exec(`
CREATE TABLE IF NOT EXISTS schema_migrations (
  name VARCHAR(255) PRIMARY KEY,
  applied_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6)
)`)
	return err
}

func migrationApplied(db *sql.DB, name string) (bool, error) {
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM schema_migrations WHERE name = ?`, name).Scan(&count); err != nil {
		return false, err
	}
	return count > 0, nil
}

func recordMigration(db *sql.DB, name string) error {
	_, err := db.Exec(`INSERT INTO schema_migrations (name) VALUES (?)`, name)
	return err
}

func countExistingTables(db *sql.DB, names []string) (int, error) {
	count := 0

	for _, name := range names {
		var tableName string
		err := db.QueryRow(`SELECT table_name FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = ? LIMIT 1`, name).Scan(&tableName)
		if err == nil {
			count++
			continue
		}
		if err == sql.ErrNoRows {
			continue
		}
		return 0, err
	}

	return count, nil
}
