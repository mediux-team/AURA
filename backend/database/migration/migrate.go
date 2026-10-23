package migration

import (
	"aura/database"
	"aura/logging"
	"context"
)

func RunMigrations() (migrationsPerformed int, Err logging.LogErrorInfo) {
	ctx, ld := logging.CreateLoggingContext(context.Background(), "Database Migration")
	defer ld.Log()
	logAction := ld.AddAction("Running Database Migrations", logging.LevelInfo)
	ctx = logging.WithCurrentAction(ctx, logAction)
	defer logAction.Complete()

	migrationsPerformed = 0
	Err = logging.LogErrorInfo{}

	// First we need to get the current VERSION of the database
	currentVersion, getCurrentVersionErr := database.GetCurrentVersion(ctx)
	if getCurrentVersionErr.Message != "" {
		return migrationsPerformed, getCurrentVersionErr
	}

	// If the current version is already the latest, nothing to do
	if currentVersion == database.LATEST_DB_VERSION {
		return migrationsPerformed, Err
	}

	// Run migrations as needed
	for v := currentVersion; v < database.LATEST_DB_VERSION; v++ {
		migrateErr := logging.LogErrorInfo{}
		switch v {
		case 0:
			migrateErr = migrate_0_to_1(ctx)
			if migrateErr.Message != "" {
				return migrationsPerformed, migrateErr
			}
			migrationsPerformed++
		case 1:
			migrateErr = migrate_1_to_2(ctx)
			if migrateErr.Message != "" {
				return migrationsPerformed, migrateErr
			}
			migrationsPerformed++
		case 2:
			migrateErr = migrate_2_to_3(ctx)
			if migrateErr.Message != "" {
				return migrationsPerformed, migrateErr
			}
			migrationsPerformed++
		case 3:
			migrateErr = migrate_3_to_4(ctx)
			if migrateErr.Message != "" {
				return migrationsPerformed, migrateErr
			}
			migrationsPerformed++
		case 4:
			migrateErr = migrate_4_to_5(ctx)
			if migrateErr.Message != "" {
				return migrationsPerformed, migrateErr
			}
			migrationsPerformed++
		case 5:
			migrateErr = migrate_5_to_6(ctx)
			if migrateErr.Message != "" {
				return migrationsPerformed, migrateErr
			}
			migrationsPerformed++
		case 6:
			migrateErr = migrate_6_to_7(ctx)
			if migrateErr.Message != "" {
				return migrationsPerformed, migrateErr
			}
			migrationsPerformed++
		case 7:
			migrateErr = migrate_7_to_8(ctx)
			if migrateErr.Message != "" {
				return migrationsPerformed, migrateErr
			}
			migrationsPerformed++
		case 8:
			migrateErr = migrate_8_to_9(ctx)
			if migrateErr.Message != "" {
				return migrationsPerformed, migrateErr
			}
			migrationsPerformed++
		case 9:
			migrateErr = migrate_9_to_10(ctx)
			if migrateErr.Message != "" {
				return migrationsPerformed, migrateErr
			}
			migrationsPerformed++
		case 10:
			migrateErr = migrate_10_to_11(ctx)
			if migrateErr.Message != "" {
				return migrationsPerformed, migrateErr
			}
			migrationsPerformed++
		case 11:
			migrateErr = migrate_11_to_12(ctx)
			if migrateErr.Message != "" {
				return migrationsPerformed, migrateErr
			}
			migrationsPerformed++
		case 12:
			migrateErr = migrate_12_to_13(ctx)
			if migrateErr.Message != "" {
				return migrationsPerformed, migrateErr
			}
			migrationsPerformed++
		case 13:
			migrateErr = migrate_13_to_14(ctx)
			if migrateErr.Message != "" {
				return migrationsPerformed, migrateErr
			}
			migrationsPerformed++
		default:
			logging.LOGGER.Error().Msgf("No migration path for database version %d", v)
			return migrationsPerformed, logging.LogErrorInfo{Message: "No migration path for database version %d"}
		}

		if migrateErr.Message == "" {
			updateErr := database.UpdateVersionTable(ctx, v+1)
			if updateErr.Message != "" {
				return migrationsPerformed, updateErr
			}
		}
	}

	return migrationsPerformed, Err
}
