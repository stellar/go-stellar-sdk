package db

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/lib/pq"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/stellar/go-stellar-sdk/support/db/dbtest"
)

func TestContextTimeoutDuringSql(t *testing.T) {
	db := dbtest.Postgres(t).Load(testSchema)
	defer db.Close()

	var cancel context.CancelFunc
	ctx := context.Background()
	ctx, cancel = context.WithTimeout(ctx, 2*time.Second)
	assert := assert.New(t)

	sessRaw := &Session{DB: db.Open()}
	reg := prometheus.NewRegistry()

	sess := RegisterMetrics(sessRaw, "test", "subtest", reg)
	defer sess.Close()
	defer cancel()

	var count int
	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		err := sess.GetRaw(ctx, &count, "SELECT pg_sleep(5) FROM people")
		assert.ErrorIs(err, ErrTimeout, "long running db server operation past context timeout, should return timeout")
		wg.Done()
	}()

	require.Eventually(t, func() bool { wg.Wait(); return true }, 5*time.Second, time.Second)
	assertDbErrorMetrics(reg, "deadline_exceeded", "57014", "user_request", assert)
}

func TestContextTimeoutBeforeSql(t *testing.T) {
	db := dbtest.Postgres(t).Load(testSchema)
	defer db.Close()

	var cancel context.CancelFunc
	ctx := context.Background()
	ctx, cancel = context.WithTimeout(ctx, time.Millisecond)
	assert := assert.New(t)

	sessRaw := &Session{DB: db.Open()}
	reg := prometheus.NewRegistry()

	sess := RegisterMetrics(sessRaw, "test", "subtest", reg)
	defer sess.Close()
	defer cancel()

	var count int
	time.Sleep(500 * time.Millisecond)
	err := sess.GetRaw(ctx, &count, "SELECT pg_sleep(5) FROM people")
	assert.ErrorIs(err, ErrTimeout, "any db server operation should return error immediately if context already timed out")
	assertDbErrorMetrics(reg, "deadline_exceeded", "deadline_exceeded", "n/a", assert)
}

func TestContextCancelledBeforeSql(t *testing.T) {
	db := dbtest.Postgres(t).Load(testSchema)
	defer db.Close()

	var cancel context.CancelFunc
	ctx := context.Background()
	ctx, cancel = context.WithCancel(ctx)
	assert := assert.New(t)

	sessRaw := &Session{DB: db.Open()}
	reg := prometheus.NewRegistry()

	sess := RegisterMetrics(sessRaw, "test", "subtest", reg)
	defer sess.Close()
	defer cancel()

	var count int
	cancel()
	err := sess.GetRaw(ctx, &count, "SELECT pg_sleep(2), COUNT(*) FROM people")
	assert.ErrorIs(err, ErrCancelled, "any db server operation should return error immediately if user already cancel")
	assertDbErrorMetrics(reg, "canceled", "canceled", "n/a", assert)
}

func TestContextCancelDuringSql(t *testing.T) {
	db := dbtest.Postgres(t).Load(testSchema)
	defer db.Close()

	var cancel context.CancelFunc
	ctx := context.Background()
	ctx, cancel = context.WithCancel(ctx)
	assert := assert.New(t)

	sessRaw := &Session{DB: db.Open()}
	reg := prometheus.NewRegistry()

	sess := RegisterMetrics(sessRaw, "test", "subtest", reg)
	defer sess.Close()
	defer cancel()

	var count int
	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		err := sess.GetRaw(ctx, &count, "SELECT pg_sleep(5) FROM people")
		assert.ErrorIs(err, ErrCancelled, "any ongoing db server operation should return error immediately after user cancel")
		wg.Done()
	}()
	time.Sleep(time.Second)
	cancel()

	require.Eventually(t, func() bool { wg.Wait(); return true }, 5*time.Second, time.Second)
	assertDbErrorMetrics(reg, "canceled", "57014", "user_request", assert)
}

func TestStatementTimeout(t *testing.T) {
	assert := assert.New(t)
	db := dbtest.Postgres(t).Load(testSchema)
	defer db.Close()

	sessRaw, err := Open(db.Dialect, db.DSN, StatementTimeout(50*time.Millisecond))
	reg := prometheus.NewRegistry()

	sess := RegisterMetrics(sessRaw, "test", "subtest", reg)
	assert.NoError(err)
	defer sess.Close()

	var count int
	err = sess.GetRaw(context.Background(), &count, "SELECT pg_sleep(2) FROM people")
	assert.ErrorIs(err, ErrStatementTimeout)
	assertDbErrorMetrics(reg, "n/a", "57014", "statement_timeout", assert)
}

func TestDeadlineOverride(t *testing.T) {
	db := dbtest.Postgres(t).Load(testSchema)
	defer db.Close()

	sess := &Session{DB: db.Open()}
	defer sess.DB.Close()

	resultCtx, _, err := sess.context(context.Background())
	assert.NoError(t, err)
	_, ok := resultCtx.Deadline()
	assert.False(t, ok)

	deadline := time.Now().Add(time.Hour)
	requestCtx := context.WithValue(context.Background(), &DeadlineCtxKey, deadline)
	resultCtx, _, err = sess.context(requestCtx)
	assert.NoError(t, err)
	d, ok := resultCtx.Deadline()
	assert.True(t, ok)
	assert.Equal(t, deadline, d)

	requestCtx, cancel := context.WithDeadline(requestCtx, time.Now().Add(time.Minute*30))
	resultCtx, _, err = sess.context(requestCtx)
	assert.NoError(t, err)
	d, ok = resultCtx.Deadline()
	assert.True(t, ok)
	assert.Equal(t, deadline, d)

	cancel()
	assert.NoError(t, resultCtx.Err())
	_, _, err = sess.context(requestCtx)
	assert.EqualError(t, err, "context canceled")

	var emptyTime time.Time
	resultCtx, _, err = sess.context(context.WithValue(context.Background(), &DeadlineCtxKey, emptyTime))
	assert.NoError(t, err)
	_, ok = resultCtx.Deadline()
	assert.False(t, ok)
}

func TestSession(t *testing.T) {
	db := dbtest.Postgres(t).Load(testSchema)
	defer db.Close()

	ctx := context.Background()
	assert := assert.New(t)
	require := require.New(t)
	sessRaw := &Session{DB: db.Open()}
	reg := prometheus.NewRegistry()

	sess := RegisterMetrics(sessRaw, "test", "subtest", reg)
	defer sess.Close()

	assert.Equal("postgres", sessRaw.Dialect())

	var count int
	err := sess.GetRaw(ctx, &count, "SELECT COUNT(*) FROM people")
	assert.NoError(err)
	assert.Equal(3, count)

	var names []string
	err = sess.SelectRaw(ctx, &names, "SELECT name FROM people")
	assert.NoError(err)
	assert.Len(names, 3)

	ret, err := sess.ExecRaw(ctx, "DELETE FROM people")
	assert.NoError(err)
	deleted, err := ret.RowsAffected()
	assert.NoError(err)
	assert.Equal(int64(3), deleted)

	// Test args (NOTE: there is a simple escaped arg to ensure no error is raised
	// during execution)
	db.Load(testSchema)
	var name string
	err = sess.GetRaw(ctx,
		&name,
		"SELECT name FROM people WHERE hunger_level = ? AND name != '??'",
		1000000,
	)
	assert.NoError(err)
	assert.Equal("scott", name)

	// Test NoRows
	err = sess.GetRaw(ctx,
		&name,
		"SELECT name FROM people WHERE hunger_level = ?",
		1234,
	)
	assert.True(sess.NoRows(err))

	// Test transactions
	db.Load(testSchema)
	require.NoError(sess.Begin(ctx), "begin failed")
	err = sess.GetRaw(ctx, &count, "SELECT COUNT(*) FROM people")
	assert.NoError(err)
	assert.Equal(3, count)
	_, err = sess.ExecRaw(ctx, "DELETE FROM people")
	assert.NoError(err)
	err = sess.GetRaw(ctx, &count, "SELECT COUNT(*) FROM people")
	assert.NoError(err)
	assert.Equal(0, count, "people did not appear deleted inside transaction")
	assert.NoError(sess.Rollback(), "rollback failed")

	// Ensure commit works
	require.NoError(sess.Begin(ctx), "begin failed")
	sess.ExecRaw(ctx, "DELETE FROM people")
	assert.NoError(sess.Commit(), "commit failed")
	err = sess.GetRaw(ctx, &count, "SELECT COUNT(*) FROM people")
	assert.NoError(err)
	assert.Equal(0, count)

	// ensure that selecting into a populated slice clears the slice first
	db.Load(testSchema)
	require.Len(names, 3, "ids slice was not preloaded with data")
	err = sess.SelectRaw(ctx, &names, "SELECT name FROM people limit 2")
	assert.NoError(err)
	assert.Len(names, 2)

	// Test ReplacePlaceholders
	out, err := sessRaw.ReplacePlaceholders("? = ? = ? = ??")
	if assert.NoError(err) {
		assert.Equal("$1 = $2 = $3 = ?", out)
	}

	assertZeroErrorMetrics(reg, assert)
}

func TestIdleTransactionTimeout(t *testing.T) {
	assert := assert.New(t)
	db := dbtest.Postgres(t).Load(testSchema)
	defer db.Close()

	sessRaw, err := Open(db.Dialect, db.DSN, IdleTransactionTimeout(50*time.Millisecond))
	assert.NoError(err)
	reg := prometheus.NewRegistry()
	sess := RegisterMetrics(sessRaw, "test", "subtest", reg)

	defer sess.Close()

	assert.NoError(sess.Begin(context.Background()))
	<-time.After(150 * time.Millisecond)

	var count int
	err = sess.GetRaw(context.Background(), &count, "SELECT COUNT(*) FROM people")
	assert.ErrorIs(err, ErrBadConnection)
	assertDbErrorMetrics(reg, "n/a", "driver_bad_connection", "n/a", assert)
}

func TestIdleTransactionTimeoutAndContextTimeout(t *testing.T) {
	assert := assert.New(t)
	db := dbtest.Postgres(t).Load(testSchema)
	defer db.Close()

	var cancel context.CancelFunc
	ctx := context.Background()
	ctx, cancel = context.WithTimeout(ctx, 150*time.Millisecond)

	sessRaw, err := Open(db.Dialect, db.DSN, IdleTransactionTimeout(100*time.Millisecond))
	assert.NoError(err)
	reg := prometheus.NewRegistry()
	sess := RegisterMetrics(sessRaw, "test", "subtest", reg)

	defer sess.Close()
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(1)
	assert.NoError(sess.Begin(context.Background()))

	<-time.After(200 * time.Millisecond)

	go func() {
		_, err := sess.ExecRaw(ctx, "SELECT pg_sleep(5) FROM people")
		assert.ErrorIs(err, ErrTimeout, "long running db server operation past context timeout, should return timeout")
		wg.Done()
	}()

	require.Eventually(t, func() bool { wg.Wait(); return true }, 5*time.Second, time.Second)
	// this demonstrates subtley of libpq error handling:
	// first a server session was created
	// 100ms elapsed and idle server session was triggered on server side, server sent signal back to libpq, libpq marks the session locally as bad
	// 150ms caller ctx deadlined
	// now caller invokes libpq and tries to submit a sql statement on the now-closed session with deadlined ctx also
	// libpq only reports an error of deadline exceeded it will not emit the driver_bad_connection due to closed server session
	assertDbErrorMetrics(reg, "deadline_exceeded", "deadline_exceeded", "n/a", assert)
}

func TestDbServerErrorCodeInMetrics(t *testing.T) {
	assert := assert.New(t)
	db := dbtest.Postgres(t).Load(testSchema)
	defer db.Close()

	sessRaw := &Session{DB: db.Open()}
	reg := prometheus.NewRegistry()
	sess := RegisterMetrics(sessRaw, "test", "subtest", reg)

	defer sess.Close()
	var pqErr *pq.Error

	_, err := sess.ExecRaw(context.Background(), "oops, invalid sql")
	assert.ErrorAs(err, &pqErr)
	assertDbErrorMetrics(reg, "n/a", "42601", "n/a", assert)
}

func TestDbOtherErrorInMetrics(t *testing.T) {
	assert := assert.New(t)
	db := dbtest.Postgres(t).Load(testSchema)
	defer db.Close()

	conn := db.Open()
	conn.Close()
	sessRaw := &Session{DB: conn}
	reg := prometheus.NewRegistry()
	sess := RegisterMetrics(sessRaw, "test", "subtest", reg)

	defer sess.Close()

	var count int
	err := sess.GetRaw(context.Background(), &count, "SELECT COUNT(*) FROM people")
	assert.ErrorContains(err, "sql: database is closed")
	assertDbErrorMetrics(reg, "n/a", "other", "n/a", assert)
}

func TestSessionAfterRollback(t *testing.T) {
	assert := assert.New(t)
	db := dbtest.Postgres(t).Load(testSchema)
	defer db.Close()

	sessRaw := setupRolledbackTx(t, db)
	reg := prometheus.NewRegistry()
	sess := RegisterMetrics(sessRaw, "test", "subtest", reg)
	defer sess.Close()

	var count int
	err := sess.GetRaw(context.Background(), &count, "SELECT COUNT(*) FROM people")
	assert.ErrorIs(err, ErrAlreadyRolledback)
	assertDbErrorMetrics(reg, "n/a", "tx_already_rollback", "n/a", assert)
}

func assertZeroErrorMetrics(reg *prometheus.Registry, assert *assert.Assertions) {
	metrics, err := reg.Gather()
	assert.NoError(err)

	for _, metricFamily := range metrics {
		if metricFamily.GetName() == "test_db_error_total" {
			assert.Fail("error_total metrics should not be present, never incremented")
		}
	}

}

func assertDbErrorMetrics(reg *prometheus.Registry, assertCtxError, assertDbError, assertDbErrorExtra string, assert *assert.Assertions) {
	metrics, err := reg.Gather()
	assert.NoError(err)

	for _, metricFamily := range metrics {
		if metricFamily.GetName() == "test_db_error_total" {
			assert.Len(metricFamily.GetMetric(), 1)
			assert.Equal(metricFamily.GetMetric()[0].GetCounter().GetValue(), float64(1))
			var ctxError = ""
			var dbError = ""
			var dbErrorExtra = ""
			for _, label := range metricFamily.GetMetric()[0].GetLabel() {
				if label.GetName() == "ctx_error" {
					ctxError = label.GetValue()
				}
				if label.GetName() == "db_error" {
					dbError = label.GetValue()
				}
				if label.GetName() == "db_error_extra" {
					dbErrorExtra = label.GetValue()
				}
			}

			assert.Equal(ctxError, assertCtxError)
			assert.Equal(dbError, assertDbError)
			assert.Equal(dbErrorExtra, assertDbErrorExtra)
			return
		}
	}
	assert.Fail("error_total metrics were not correct")
}

func setupRolledbackTx(t *testing.T, db *dbtest.DB) *Session {
	var cancel context.CancelFunc
	ctx := context.Background()
	ctx, cancel = context.WithCancel(ctx)

	sess := &Session{DB: db.Open()}

	assert.NoError(t, sess.Begin(ctx))

	var count int
	assert.NoError(t, sess.GetRaw(ctx, &count, "SELECT COUNT(*) FROM people"))
	assert.Equal(t, 3, count)

	cancel()
	time.Sleep(500 * time.Millisecond)
	return sess
}
