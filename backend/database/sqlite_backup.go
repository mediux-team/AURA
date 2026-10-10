package database

import (
	"aura/config"
	"aura/logging"
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"time"
)

func (s *SQliteDB) Backup(ctx context.Context, currentVersion, newVersion int) (Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, "Backing up SQLite Database", logging.LevelInfo)
	defer logAction.Complete()

	Err = logging.LogErrorInfo{}

	// Build DSN
	dsn, Err := BuildDSN()
	if Err.Message != "" {
		return Err
	}

	// Construct the DB path
	dbPath := path.Join(config.ConfigPath, dsn)

	// Check if the database file exists
	// If it doesn't exist, there's nothing to back up
	// This can happen on first run
	// So we just return
	_, statErr := os.Stat(dbPath)
	if os.IsNotExist(statErr) {
		return Err
	} else if statErr != nil {
		logAction.SetError("Failed to stat database file for backup",
			"Ensure the database path is accessible.",
			map[string]any{
				"error": statErr.Error(),
				"path":  dbPath,
			})
		return *logAction.Error
	}

	// Committed writes can still be in the WAL file; move them into the main file before copying it.
	// A checkpoint blocked by another connection is reported in busy, not as an error.
	var busy, walFrames, checkpointedFrames int
	err := s.conn.QueryRowContext(ctx, "PRAGMA wal_checkpoint(TRUNCATE)").Scan(&busy, &walFrames, &checkpointedFrames)
	if err == nil && busy != 0 {
		err = fmt.Errorf("checkpoint blocked by another connection")
	}
	if err != nil {
		logAction.SetError("Failed to checkpoint database before backup",
			"Ensure no other process is holding the database open.",
			map[string]any{
				"error": err.Error(),
				"path":  dbPath,
			})
		return *logAction.Error
	}

	// Create a backup file with version and timestamp
	// Create a backup file path with version and timestamp
	timestamp := time.Now().Format("20060102_150405")
	backupPath := fmt.Sprintf("%s_backup_v%d_to_v%d_%s.db", dbPath[:len(dbPath)-3], currentVersion, newVersion, timestamp)

	// Copy the database file to the backup location
	actionBackupFile := logAction.AddSubAction("Backing up database file", logging.LevelTrace)
	sourceFile, err := os.Open(dbPath)
	if err != nil {
		actionBackupFile.SetError("Failed to open source database file for backup",
			"Ensure the database path is accessible.",
			map[string]any{
				"error": err.Error(),
				"path":  dbPath,
			})
		return *actionBackupFile.Error
	}
	defer sourceFile.Close()

	// Create the backup file
	backupFile, err := os.Create(backupPath)
	if err != nil {
		actionBackupFile.SetError("Failed to create backup database file",
			"Ensure the destination path is accessible and writable.",
			map[string]any{
				"error": err.Error(),
				"path":  backupPath,
			})
		return *actionBackupFile.Error
	}
	defer backupFile.Close()

	// Copy the contents
	_, err = io.Copy(backupFile, sourceFile)
	if err != nil {
		actionBackupFile.SetError("Failed to copy database file for backup",
			"Ensure there is sufficient disk space and permissions.",
			map[string]any{
				"error":       err.Error(),
				"source":      dbPath,
				"destination": backupPath,
			})
		return *actionBackupFile.Error
	}
	actionBackupFile.Complete()

	return Err
}
