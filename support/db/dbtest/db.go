package dbtest

import (
	"context"
	"crypto/rand"
	"database/sql"

	"encoding/hex"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
	"github.com/stellar/go-stellar-sdk/support/db/sqlutils"
	"github.com/stellar/go-stellar-sdk/support/errors"
	"github.com/stretchr/testify/require"
)

// DB represents an ephemeral database that starts blank and can be used
// to run tests against.
type DB struct {
	Dialect string
	DSN     string
	RO_DSN  string
	dbName  string
	t       testing.TB
	closer  func()
	closed  bool
}

// randomName returns a new psuedo-random name that is sufficient for naming a
// test database.  In the event that reading from the source of randomness
// fails, a panic will occur.
func randomName() string {
	raw := make([]byte, 6)

	_, err := rand.Read(raw)
	if err != nil {
		err = errors.Wrap(err, "read from rand failed")
		panic(err)
	}

	enc := hex.EncodeToString(raw)

	return fmt.Sprintf("test_%s", enc)
}

// Close closes and deletes the database represented by `db`
func (db *DB) Close() {
	if db.closed {
		return
	}

	db.closer()
	db.closed = true
}

// Load executes all of the statements in the provided sql script against the
// test database, panicking if any fail.  The receiver is returned allowing for
// chain-style calling within your test functions.
func (db *DB) Load(sql string) *DB {
	conn := db.Open()
	defer conn.Close()

	tx, err := conn.Begin()
	require.NoError(db.t, err)

	defer tx.Rollback()

	for i, cmd := range sqlutils.AllStatements(sql) {
		_, err = tx.Exec(cmd)
		require.NoError(db.t, err, "failed execing statement: %d", i)
	}

	err = tx.Commit()
	require.NoError(db.t, err)

	return db
}

// Open opens a sqlx connection to the db.
func (db *DB) Open() *sqlx.DB {
	conn, err := sqlx.Open(db.Dialect, db.DSN)
	require.NoError(db.t, err)

	return conn
}

func (db *DB) Version() (major int) {
	conn := db.Open()
	defer conn.Close()

	versionFull := ""
	err := conn.Get(&versionFull, "SHOW server_version")
	require.NoError(db.t, err)

	version := strings.Fields(versionFull)
	parts := strings.Split(version[0], ".")
	major, err = strconv.Atoi(parts[0])
	require.NoError(db.t, err)

	return major
}

func execStatement(t testing.TB, query string, DSN string) {
	db, err := sqlx.Open("postgres", DSN)
	require.NoError(t, err)
	_, err = db.Exec(query)
	require.NoError(t, err)
	require.NoError(t, db.Close())
}

func checkReadOnly(t testing.TB, DSN string) {
	conn, err := sqlx.Open("postgres", DSN)
	require.NoError(t, err)
	defer conn.Close()

	tx, err := conn.BeginTx(context.Background(), &sql.TxOptions{})
	require.NoError(t, err)
	defer tx.Rollback()

	rows, err := tx.Query("SELECT FROM pg_user WHERE  usename = 'user_ro'")
	require.NoError(t, err)

	if !rows.Next() {
		_, err = tx.Exec("CREATE ROLE user_ro WITH LOGIN PASSWORD 'user_ro';")
		if err != nil {
			// Handle race condition by ignoring the error if it's a duplicate key violation or duplicate object error
			if pqErr, ok := err.(*pq.Error); ok && (pqErr.Code == "23505" || pqErr.Code == "42710") {
				return
			} else if ok {
				t.Logf("pq error code: %s", pqErr.Code)
			}
		}
		require.NoError(t, err)
	}

	err = tx.Commit()
	require.NoError(t, err)
}

// Postgres provisions a new, blank database with a random name on the localhost
// of the running process.  It assumes that you have postgres running on the
// default port, have the command line postgres tools installed, and that the
// current user has access to the server.  It panics on the event of a failure.
func Postgres(t testing.TB) *DB {
	var result DB
	result.dbName = randomName()
	result.Dialect = "postgres"
	result.t = t

	t.Log("Test Database:", result.dbName)

	pgUser := os.Getenv("PGUSER")
	if len(pgUser) == 0 {
		pgUser = "postgres"
	}
	pgPwd := os.Getenv("PGPASSWORD")
	if len(pgPwd) == 0 {
		pgPwd = "postgres"
	}

	postgresDSN := fmt.Sprintf("postgres://%s:%s@localhost/?sslmode=disable", pgUser, pgPwd)
	result.DSN = fmt.Sprintf("postgres://%s:%s@localhost/%s?sslmode=disable&timezone=UTC", pgUser, pgPwd, result.dbName)
	result.RO_DSN = fmt.Sprintf("postgres://%s:%s@localhost/%s?sslmode=disable&timezone=UTC", "user_ro", "user_ro", result.dbName)

	execStatement(t, fmt.Sprintf("CREATE DATABASE %s;", result.dbName), postgresDSN)
	execStatement(t, fmt.Sprintf("GRANT CONNECT ON DATABASE %s TO PUBLIC;", result.dbName), postgresDSN)
	execStatement(t, "GRANT USAGE ON SCHEMA public TO PUBLIC;", result.DSN)
	execStatement(t, "GRANT SELECT ON ALL TABLES IN SCHEMA public TO PUBLIC;", result.DSN)
	execStatement(t, "ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT SELECT ON TABLES TO PUBLIC;", result.DSN)

	checkReadOnly(t, postgresDSN)

	result.closer = func() {
		// pg_terminate_backend is a best effort, it does not gaurantee that it can close any lingering connections
		// it sends a quit signal to each remaining connection in the db
		execStatement(t, "SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = '"+pq.QuoteIdentifier(result.dbName)+"';", postgresDSN)
		execStatement(t, "DROP DATABASE "+pq.QuoteIdentifier(result.dbName), postgresDSN)
	}

	return &result
}
