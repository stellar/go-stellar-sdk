// Package db is the base package for database access at stellar.  It primarily
// exposes Session which is a lightweight wrapper around a *sqlx.DB that
// provides utility methods (See the repo tests for examples).
//
// In addition to the query methods, this package provides query logging and
// stateful transaction management.
//
// In addition to the lower-level access facilities, this package exposes a
// system to build queries more dynamically using the help of
// https://github.com/Masterminds/squirrel.  These builder method are access
// through the `Table` type.
package db

import (
	"context"
	"database/sql"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/jmoiron/sqlx"

	"github.com/stellar/go-stellar-sdk/support/errors"

	// Enable postgres
	_ "github.com/lib/pq"
)

const (
	// PostgresQueryMaxParams defines the maximum number of parameters in a query.
	PostgresQueryMaxParams = 65535
	maxDBPingAttempts      = 30
)

var (
	// ErrTimeout is an error returned by Session methods when request has
	// taken longer than context's deadline max duration
	ErrTimeout = errors.New("canceling statement due to lack of response within timeout period")
	// ErrCancelled is an error returned by Session methods when request has
	// been canceled (ex. context canceled).
	ErrCancelled = errors.New("canceling statement due to user request")
	// ErrAlreadyRolledback is an error returned by Session methods when the transaction
	// containing the request has already been rolled back.
	ErrAlreadyRolledback = errors.New("transaction has already been committed or rolled back")
	// ErrConflictWithRecovery is an error returned by Session methods when
	// read replica cancels the query due to conflict with about-to-be-applied
	// WAL entries (https://www.postgresql.org/docs/current/hot-standby.html).
	ErrConflictWithRecovery = errors.New("canceling statement due to conflict with recovery")
	// ErrBadConnection is an error returned when driver returns `bad connection`
	// error.
	ErrBadConnection = errors.New("bad connection")
	// ErrStatementTimeout is an error returned by Session methods when request has
	// been canceled due to a statement timeout.
	ErrStatementTimeout = errors.New("canceling statement due to statement timeout")
)

// Conn represents a connection to a single database.
type Conn interface {
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
	GetContext(ctx context.Context, dest interface{}, query string, args ...interface{}) error
	Rebind(sql string) string
	QueryxContext(ctx context.Context, query string, args ...interface{}) (*sqlx.Rows, error)
	SelectContext(ctx context.Context, dest interface{}, query string, args ...interface{}) error
}

// DeleteBuilder is a helper struct used to construct sql queries of the DELETE
// variety.
type DeleteBuilder struct {
	Table *Table
	sql   squirrel.DeleteBuilder
}

// InsertBuilder is a helper struct used to construct sql queries of the INSERT
// variety.
// NOTE: InsertBuilder will use "zero" value of a type in case of nil pointer values.
// If you need to insert `NULL` use sql.Null* or build your own type that implements
// database/sql/driver.Valuer.
type InsertBuilder struct {
	Table *Table

	rows        []interface{}
	ignoredCols map[string]bool
	sql         squirrel.InsertBuilder
}

// GetBuilder is a helper struct used to construct sql queries of the SELECT
// variety.
type GetBuilder struct {
	Table *Table

	dest interface{}
	sql  squirrel.SelectBuilder
}

// SelectBuilder is a helper struct used to construct sql queries of the SELECT
// variety.
type SelectBuilder struct {
	Table *Table

	dest interface{}
	sql  squirrel.SelectBuilder
}

// UpdateBuilder is a helper struct used to construct sql queries of the UPDATE
// variety.
type UpdateBuilder struct {
	Table *Table

	sql squirrel.UpdateBuilder
}

// Session provides helper methods for making queries against `DB` and provides
// utilities such as automatic query logging and transaction management. NOTE:
// Because transaction-handling is stateful, it is not presently intended to
// cross goroutine boundaries and is not concurrency safe.
type Session struct {
	// DB is the database connection that queries should be executed against.
	DB *sqlx.DB

	tx            *sqlx.Tx
	txCancel      context.CancelFunc
	txOptions     *sql.TxOptions
	errorHandlers []ErrorHandlerFunc
}

// dbErr - the Postgres error
// ctx   - the caller's context
type ErrorHandlerFunc func(dbErr error, ctx context.Context)
type SessionInterface interface {
	BeginTx(ctx context.Context, opts *sql.TxOptions) error
	Begin(ctx context.Context) error
	Rollback() error
	Commit() error
	GetTx() *sqlx.Tx
	GetTxOptions() *sql.TxOptions
	TruncateTables(ctx context.Context, tables []string) error
	Clone() SessionInterface
	Close() error
	Get(ctx context.Context, dest interface{}, query squirrel.Sqlizer) error
	GetRaw(ctx context.Context, dest interface{}, query string, args ...interface{}) error
	Select(ctx context.Context, dest interface{}, query squirrel.Sqlizer) error
	SelectRaw(ctx context.Context, dest interface{}, query string, args ...interface{}) error
	Query(ctx context.Context, query squirrel.Sqlizer) (*Rows, error)
	QueryRaw(ctx context.Context, query string, args ...interface{}) (*Rows, error)
	GetTable(name string) *Table
	Exec(ctx context.Context, query squirrel.Sqlizer) (sql.Result, error)
	ExecRaw(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
	NoRows(err error) bool
	Ping(ctx context.Context, timeout time.Duration) error
	DeleteRange(
		ctx context.Context,
		start, end int64,
		table string,
		idCol string,
	) (int64, error)
}

// Table helps to build sql queries against a given table.  It logically
// represents a SQL table on the database that `Session` is connected to.
type Table struct {
	// Name is the name of the table
	Name string

	Session *Session
}

func pingDB(db *sqlx.DB) error {
	var err error
	for attempt := 0; attempt < maxDBPingAttempts; attempt++ {
		if err = db.Ping(); err == nil {
			return nil
		}
		time.Sleep(time.Second)
	}

	return errors.Wrapf(err, "failed to connect to DB after %v attempts", maxDBPingAttempts)
}

type ClientConfig struct {
	Key   string
	Value string
}

func StatementTimeout(timeout time.Duration) ClientConfig {
	return ClientConfig{
		Key:   "statement_timeout",
		Value: strconv.FormatInt(timeout.Milliseconds(), 10),
	}
}

func IdleTransactionTimeout(timeout time.Duration) ClientConfig {
	return ClientConfig{
		Key:   "idle_in_transaction_session_timeout",
		Value: strconv.FormatInt(timeout.Milliseconds(), 10),
	}
}

func augmentDSN(dsn string, clientConfigs []ClientConfig) string {
	parsed, err := url.Parse(dsn)
	// dsn can either be a postgres url like "postgres://postgres:123456@127.0.0.1:5432"
	// or, it can be a white space separated string of key value pairs like
	// "host=localhost port=5432 user=bob password=secret"
	if err != nil || parsed.Scheme == "" {
		// if dsn does not parse as a postgres url, we assume it must be take
		// the form of a white space separated string
		parts := []string{dsn}
		for _, config := range clientConfigs {
			// do not override if the key is already present in dsn
			if strings.Contains(dsn, config.Key+"=") {
				continue
			}
			parts = append(parts, config.Key+"="+config.Value)
		}
		return strings.Join(parts, " ")
	}

	q := parsed.Query()
	for _, config := range clientConfigs {
		// do not override if the key is already present in dsn
		if len(q.Get(config.Key)) > 0 {
			continue
		}
		q.Set(config.Key, config.Value)
	}
	parsed.RawQuery = q.Encode()
	return parsed.String()
}

// Open the database at `dsn` and returns a new *Session using it.
func Open(dialect, dsn string, clientConfigs ...ClientConfig) (*Session, error) {
	dsn = augmentDSN(dsn, clientConfigs)
	db, err := sqlx.Open(dialect, dsn)
	if err != nil {
		return nil, errors.Wrap(err, "open failed")
	}
	if err = pingDB(db); err != nil {
		return nil, errors.Wrap(err, "ping failed")
	}

	return &Session{DB: db}, nil
}

// Wrap wraps a bare *sql.DB (from the database/sql stdlib package) in a
// *db.Session instance.  It is meant to be used in cases where you do not
// control the instantiation of the database connection, but would still like to
// leverage the facilities provided in Session.
func Wrap(base *sql.DB, dialect string) *Session {
	return &Session{DB: sqlx.NewDb(base, dialect)}
}

// ensure various types conform to Conn interface
var _ Conn = (*sqlx.Tx)(nil)
var _ Conn = (*sqlx.DB)(nil)
