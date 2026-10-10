package database

import (
	"aura/config"
	"aura/logging"
	"context"
	"database/sql"
	"path/filepath"
	"testing"
)

func TestBackupIncludesWritesStillInWAL(t *testing.T) {
	conn := openTestDB(t)
	if _, err := conn.Exec("CREATE TABLE items (id INTEGER PRIMARY KEY)"); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec("INSERT INTO items (id) VALUES (1), (2), (3)"); err != nil {
		t.Fatal(err)
	}

	ctx, ld := logging.CreateLoggingContext(context.Background(), "test")
	ctx = logging.WithCurrentAction(ctx, ld.AddAction("test", logging.LevelInfo))
	if Err := (&SQliteDB{conn: conn}).Backup(ctx, 1, 2); Err.Message != "" {
		t.Fatalf("Backup: %s", Err.Message)
	}

	matches, err := filepath.Glob(filepath.Join(config.ConfigPath, "test_backup_v1_to_v2_*.db"))
	if err != nil || len(matches) != 1 {
		t.Fatalf("expected one backup file, got %v (err %v)", matches, err)
	}
	backup, err := sql.Open("sqlite3", matches[0])
	if err != nil {
		t.Fatal(err)
	}
	defer backup.Close()

	var count int
	if err := backup.QueryRow("SELECT COUNT(*) FROM items").Scan(&count); err != nil {
		t.Fatalf("reading backup: %v", err)
	}
	if count != 3 {
		t.Errorf("backup has %d rows, want 3", count)
	}
}
