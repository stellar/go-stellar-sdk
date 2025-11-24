package db

import (
	"context"
	"fmt"
	"testing"

	"github.com/stellar/go-stellar-sdk/support/db/dbtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type hungerRow struct {
	Name        string `db:"name"`
	HungerLevel string `db:"hunger_level"`
	JsonValue   string `db:"json_value"`
}

type invalidHungerRow struct {
	Name        string `db:"name"`
	HungerLevel string `db:"hunger_level"`
	LastName    string `db:"last_name"`
}

func BenchmarkBatchInsertBuilder(b *testing.B) {
	// In order to show SQL queries
	// log.SetLevel(logrus.DebugLevel)
	db := dbtest.Postgres(b).Load(testSchema)
	defer db.Close()
	sess := &Session{DB: db.Open()}
	defer sess.DB.Close()
	ctx := context.Background()
	maxBatchSize := 1000
	insertBuilder := &BatchInsertBuilder{
		Table:        sess.GetTable("people"),
		MaxBatchSize: maxBatchSize,
	}

	// Do not count the test initialization
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for j := 0; j < maxBatchSize; j++ {
			err := insertBuilder.RowStruct(ctx, hungerRow{
				Name:        fmt.Sprintf("bubba%d", i*maxBatchSize+j),
				HungerLevel: "1202",
			})
			require.NoError(b, err)
		}
	}
	err := insertBuilder.Exec(ctx)

	// Do not count the test ending
	b.StopTimer()
	assert.NoError(b, err)
	var count []int
	err = sess.SelectRaw(ctx,
		&count,
		"SELECT COUNT(*) FROM people",
	)
	assert.NoError(b, err)
	preexistingCount := 3
	assert.Equal(b, b.N*maxBatchSize+preexistingCount, count[0])
}

func TestBatchInsertBuilder(t *testing.T) {
	db := dbtest.Postgres(t).Load(testSchema)
	defer db.Close()
	sess := &Session{DB: db.Open()}
	defer sess.DB.Close()
	ctx := context.Background()

	insertBuilder := &BatchInsertBuilder{
		Table: sess.GetTable("people"),
	}

	// exec on the empty set should produce no errors
	assert.NoError(t, insertBuilder.Exec(ctx))

	var err error

	err = insertBuilder.Row(ctx, map[string]interface{}{
		"name":         "bubba",
		"hunger_level": "120",
	})
	assert.NoError(t, err)

	err = insertBuilder.RowStruct(ctx, hungerRow{
		Name:        "bubba2",
		HungerLevel: "1202",
	})
	assert.NoError(t, err)

	// Extra column
	err = insertBuilder.Row(ctx, map[string]interface{}{
		"name":         "bubba",
		"hunger_level": "120",
		"abc":          "def",
	})
	assert.EqualError(t, err, "invalid number of columns (expected=2, actual=3)")

	// Not enough columns
	err = insertBuilder.Row(ctx, map[string]interface{}{
		"name": "bubba",
	})
	assert.EqualError(t, err, "invalid number of columns (expected=2, actual=1)")

	// Invalid column
	err = insertBuilder.Row(ctx, map[string]interface{}{
		"name":  "bubba",
		"hello": "120",
	})
	assert.EqualError(t, err, `column "hunger_level" does not exist`)

	err = insertBuilder.RowStruct(ctx, invalidHungerRow{
		Name:        "Max",
		HungerLevel: "500",
	})
	assert.EqualError(t, err, `expected value of type "db.hungerRow" but got "db.invalidHungerRow" value`)

	err = insertBuilder.Exec(ctx)
	assert.NoError(t, err)

	// Check rows
	var found []person
	err = sess.SelectRaw(ctx, &found, `SELECT * FROM people WHERE name like 'bubba%'`)

	require.NoError(t, err)
	assert.Equal(
		t,
		found,
		[]person{
			{Name: "bubba", HungerLevel: "120"},
			{Name: "bubba2", HungerLevel: "1202"},
		},
	)

	err = insertBuilder.Row(ctx, map[string]interface{}{
		"name":         "bubba",
		"hunger_level": "1",
	})
	assert.NoError(t, err)

	err = insertBuilder.Exec(ctx)
	assert.EqualError(
		t, err, "error adding values while inserting to people: exec failed: pq:"+
			" duplicate key value violates unique constraint \"people_pkey\"",
	)

	insertBuilder.Suffix = "ON CONFLICT (name) DO NOTHING"

	err = insertBuilder.Row(ctx, map[string]interface{}{
		"name":         "bubba",
		"hunger_level": "1",
	})
	assert.NoError(t, err)

	err = insertBuilder.Exec(ctx)
	assert.NoError(t, err)

	err = sess.SelectRaw(ctx, &found, `SELECT * FROM people WHERE name like 'bubba%'`)

	require.NoError(t, err)
	assert.Equal(
		t,
		found,
		[]person{
			{Name: "bubba", HungerLevel: "120"},
			{Name: "bubba2", HungerLevel: "1202"},
		},
	)

	insertBuilder.Suffix = "ON CONFLICT (name) DO UPDATE SET hunger_level = EXCLUDED.hunger_level"

	err = insertBuilder.Row(ctx, map[string]interface{}{
		"name":         "bubba",
		"hunger_level": "1",
	})
	assert.NoError(t, err)

	err = insertBuilder.Exec(ctx)
	assert.NoError(t, err)

	err = sess.SelectRaw(ctx, &found, `SELECT * FROM people WHERE name like 'bubba%' order by name desc`)

	require.NoError(t, err)
	assert.Equal(
		t,
		found,
		[]person{
			{Name: "bubba2", HungerLevel: "1202"},
			{Name: "bubba", HungerLevel: "1"},
		},
	)
}
