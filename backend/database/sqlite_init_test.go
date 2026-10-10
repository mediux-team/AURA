package database

import (
	"aura/config"
	"aura/logging"
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dir := t.TempDir()
	config.ConfigPath = dir
	config.Current.Database = config.Config_Database{Type: "sqlite3", Path: "test.db"}
	// Pre-create the file so GetDBConnection skips the new-database log path.
	if err := os.WriteFile(filepath.Join(dir, "test.db"), nil, 0o600); err != nil {
		t.Fatal(err)
	}

	ctx, ld := logging.CreateLoggingContext(context.Background(), "test")
	ctx = logging.WithCurrentAction(ctx, ld.AddAction("test", logging.LevelInfo))
	conn, _, Err := (&SQliteDB{}).GetDBConnection(ctx)
	if Err.Message != "" {
		t.Fatalf("GetDBConnection: %s", Err.Message)
	}
	t.Cleanup(func() { conn.Close() })
	return conn
}

func TestGetDBConnectionUsesWALAndBusyTimeout(t *testing.T) {
	conn := openTestDB(t)

	var journalMode string
	if err := conn.QueryRow("PRAGMA journal_mode").Scan(&journalMode); err != nil {
		t.Fatal(err)
	}
	if journalMode != "wal" {
		t.Errorf("journal_mode = %q, want %q", journalMode, "wal")
	}

	var busyTimeout int
	if err := conn.QueryRow("PRAGMA busy_timeout").Scan(&busyTimeout); err != nil {
		t.Fatal(err)
	}
	if busyTimeout != sqliteBusyTimeoutMs {
		t.Errorf("busy_timeout = %d, want %d", busyTimeout, sqliteBusyTimeoutMs)
	}
}

// Transactions that read before writing fail with SQLITE_BUSY under deferred
// locking, even with a busy timeout. This mirrors the queue worker and the
// Plex listener writing at the same time.
func TestConcurrentReadThenWriteTransactionsDoNotFail(t *testing.T) {
	conn := openTestDB(t)
	if _, err := conn.Exec("CREATE TABLE counter (n INTEGER NOT NULL)"); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec("INSERT INTO counter (n) VALUES (0)"); err != nil {
		t.Fatal(err)
	}

	const workers, iterations = 8, 25
	var wg sync.WaitGroup
	errs := make(chan error, workers*iterations)
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				tx, err := conn.Begin()
				if err != nil {
					errs <- fmt.Errorf("begin: %w", err)
					continue
				}
				var n int
				if err := tx.QueryRow("SELECT n FROM counter").Scan(&n); err != nil {
					tx.Rollback()
					errs <- fmt.Errorf("read: %w", err)
					continue
				}
				if _, err := tx.Exec("UPDATE counter SET n = ?", n+1); err != nil {
					tx.Rollback()
					errs <- fmt.Errorf("write: %w", err)
					continue
				}
				if err := tx.Commit(); err != nil {
					errs <- fmt.Errorf("commit: %w", err)
				}
			}
		}()
	}
	wg.Wait()
	close(errs)

	failures := 0
	for err := range errs {
		if failures < 3 {
			t.Error(err)
		}
		failures++
	}
	if failures > 0 {
		t.Fatalf("%d of %d transactions failed", failures, workers*iterations)
	}

	var n int
	if err := conn.QueryRow("SELECT n FROM counter").Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != workers*iterations {
		t.Errorf("counter = %d, want %d", n, workers*iterations)
	}
}
