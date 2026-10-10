package database

import (
	"aura/config"
	"aura/logging"
	"context"
	"database/sql"
	"fmt"
	"os"
	"path"

	_ "github.com/mattn/go-sqlite3"
)

// The download queue worker, Plex event listener and API handlers write concurrently.
// WAL stops readers blocking the writer, the busy timeout makes writers wait instead of
// failing, and immediate transactions take the write lock up front: a deferred
// transaction that upgrades from read to write fails at once with SQLITE_BUSY.
const sqliteBusyTimeoutMs = 30000

var sqliteConnectionParams = fmt.Sprintf("?_journal_mode=WAL&_busy_timeout=%d&_txlock=immediate", sqliteBusyTimeoutMs)

func (s *SQliteDB) Init(ctx context.Context) (newDB bool, Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, "Initializing Database", logging.LevelInfo)
	defer logAction.Complete()

	Err = logging.LogErrorInfo{}
	newDB = false

	// Open DB Connection
	s.conn, newDB, Err = s.GetDBConnection(ctx)
	if Err.Message != "" {
		return newDB, Err
	}

	// If new DB, create version table, main tables and set version
	if newDB {
		Err = s.CreateVersionTable(ctx)
		if Err.Message != "" {
			return newDB, Err
		}

		Err = s.CreateAuthTable(ctx)
		if Err.Message != "" {
			return newDB, Err
		}

		Err = s.CreateTables(ctx)
		if Err.Message != "" {
			return newDB, Err
		}

		Err = s.UpdateVersionTable(ctx, LATEST_DB_VERSION)
		if Err.Message != "" {
			return newDB, Err
		}
	}

	return newDB, Err
}

func (s *SQliteDB) GetConfig() (config config.Config_Database) {
	return s.Config
}

func (s *SQliteDB) GetDBConnection(ctx context.Context) (conn *sql.DB, newDB bool, Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, "Opening Database Connection", logging.LevelInfo)
	defer logAction.Complete()

	conn = nil
	newDB = false
	Err = logging.LogErrorInfo{}

	// Build DSN
	dsn, Err := BuildDSN()
	if Err.Message != "" {
		return conn, newDB, Err
	}

	// Construct the DB path
	dbPath := path.Join(config.ConfigPath, dsn)

	// Check if DB file exists
	_, statErr := os.Stat(dbPath)
	newDB = os.IsNotExist(statErr)
	if newDB {
		logging.LOGGER.Warn().Timestamp().Msg("Database file not found. Creating new database.")
	}

	conn, openErr := sql.Open("sqlite3", dbPath+sqliteConnectionParams)
	if openErr != nil {
		return conn, newDB, logging.LogErrorInfo{Message: openErr.Error()}
	}

	// Ping to verify connection
	if pingErr := conn.PingContext(ctx); pingErr != nil {
		return conn, newDB, logging.LogErrorInfo{Message: pingErr.Error()}
	}

	return conn, newDB, Err
}
