package database

import (
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/Tewan0/FairPick/config"
	"gorm.io/gorm"
)

const DefaultTestDBName = "fairpick_test"

var (
	lockDBMu sync.Mutex
	lockDBs  = make(map[*gorm.DB]*gorm.DB)
)

// ConnectTestDB connects to an isolated PostgreSQL test database (default: fairpick_test),
// creating it automatically if it does not exist yet, and runs Migrate() on it.
// It uses a dedicated lock connection for pg_advisory_lock(88442299) to serialize parallel
// `go test` package processes while keeping a multi-connection pool on testDB so concurrent
// transactions and race condition tests execute in parallel against PostgreSQL.
func ConnectTestDB() (*gorm.DB, *config.Config, error) {
	cfg, err := config.LoadConfig()
	if err != nil {
		return nil, nil, err
	}

	testDBName := strings.TrimSpace(os.Getenv("TEST_DB_NAME"))
	if testDBName == "" {
		testDBName = DefaultTestDBName
	}
	if strings.EqualFold(testDBName, "fairpick") {
		return nil, nil, fmt.Errorf("safety check failed: TEST_DB_NAME must not be 'fairpick'")
	}

	// Connect to existing DB first to ensure test database exists
	adminDB, err := Connect(cfg)
	if err != nil {
		return nil, nil, err
	}

	var exists bool
	if err := adminDB.Raw(`SELECT EXISTS (SELECT 1 FROM pg_database WHERE datname = ?)`, testDBName).Scan(&exists).Error; err != nil {
		return nil, nil, fmt.Errorf("failed checking existence of test database %s: %w", testDBName, err)
	}

	if !exists {
		createSQL := fmt.Sprintf(`CREATE DATABASE "%s"`, strings.ReplaceAll(testDBName, `"`, `""`))
		if err := adminDB.Exec(createSQL).Error; err != nil {
			if !strings.Contains(err.Error(), "23505") && !strings.Contains(err.Error(), "42P04") && !strings.Contains(err.Error(), "already exists") {
				return nil, nil, fmt.Errorf("failed creating test database %s: %w", testDBName, err)
			}
		}
	}

	if sqlAdmin, err := adminDB.DB(); err == nil {
		_ = sqlAdmin.Close()
	}

	testCfg := *cfg
	testCfg.DBName = testDBName

	// 1. Open dedicated 1-connection handle to hold the cross-process PostgreSQL session advisory lock
	lockConn, err := Connect(&testCfg)
	if err != nil {
		return nil, nil, fmt.Errorf("failed connecting lock session to test database %s: %w", testDBName, err)
	}
	if sqlLock, err := lockConn.DB(); err == nil {
		sqlLock.SetMaxOpenConns(1)
		sqlLock.SetMaxIdleConns(1)
	}
	if err := lockConn.Exec("SELECT pg_advisory_lock(88442299)").Error; err != nil {
		if sqlLock, e := lockConn.DB(); e == nil {
			_ = sqlLock.Close()
		}
		return nil, nil, fmt.Errorf("failed acquiring test db advisory lock: %w", err)
	}

	// 2. Open the main testDB with a full connection pool (25 max open conns) for concurrent requests/transactions
	testDB, err := Connect(&testCfg)
	if err != nil {
		_ = lockConn.Exec("SELECT pg_advisory_unlock_all()")
		if sqlLock, e := lockConn.DB(); e == nil {
			_ = sqlLock.Close()
		}
		return nil, nil, fmt.Errorf("failed connecting to test database %s: %w", testDBName, err)
	}
	if sqlTest, err := testDB.DB(); err == nil {
		sqlTest.SetMaxOpenConns(25)
		sqlTest.SetMaxIdleConns(10)
	}

	if err := Migrate(testDB); err != nil {
		_ = lockConn.Exec("SELECT pg_advisory_unlock_all()")
		if sqlLock, e := lockConn.DB(); e == nil {
			_ = sqlLock.Close()
		}
		if sqlTest, e := testDB.DB(); e == nil {
			_ = sqlTest.Close()
		}
		return nil, nil, fmt.Errorf("failed migrating test database %s: %w", testDBName, err)
	}

	lockDBMu.Lock()
	lockDBs[testDB] = lockConn
	lockDBMu.Unlock()

	return testDB, &testCfg, nil
}

// CloseTestDB closes the test database pool and releases the cross-process advisory lock connection.
func CloseTestDB(db *gorm.DB) {
	if db == nil {
		return
	}

	lockDBMu.Lock()
	lockConn, ok := lockDBs[db]
	if ok {
		delete(lockDBs, db)
	}
	lockDBMu.Unlock()

	if sqlTest, err := db.DB(); err == nil {
		_ = sqlTest.Close()
	}

	if ok && lockConn != nil {
		_ = lockConn.Exec("SELECT pg_advisory_unlock_all()")
		if sqlLock, err := lockConn.DB(); err == nil {
			_ = sqlLock.Close()
		}
	}
}

// ResetTestData truncates all 6 tables in the test database cleanly.
// Refuses to run if the connected database is "fairpick".
func ResetTestData(db *gorm.DB) error {
	var currentDB string
	if err := db.Raw(`SELECT current_database()`).Scan(&currentDB).Error; err != nil {
		return err
	}
	if strings.EqualFold(strings.TrimSpace(currentDB), "fairpick") {
		return fmt.Errorf("safety check failed: refusing to truncate main 'fairpick' database")
	}

	return db.Exec(`
		TRUNCATE TABLE
			pick_history,
			attendance,
			class_sessions,
			enrollments,
			students,
			courses
		RESTART IDENTITY CASCADE;
	`).Error
}
