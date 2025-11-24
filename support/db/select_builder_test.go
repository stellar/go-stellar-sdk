package db

import (
	"context"
	"testing"

	"github.com/stellar/go-stellar-sdk/support/db/dbtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSelectBuilder_Exec(t *testing.T) {
	db := dbtest.Postgres(t).Load(testSchema)
	defer db.Close()
	sess := &Session{DB: db.Open()}
	defer sess.DB.Close()

	var results []person

	tbl := sess.GetTable("people")
	sb := tbl.Select(&results, "name = ?", "scott")
	sql, args, err := sb.sql.ToSql()
	require.NoError(t, err)

	assert.Contains(t, sql, "name")
	assert.Contains(t, sql, "hunger_level")
	assert.NotContains(t, sql, "-")

	if assert.Len(t, args, 1) {
		assert.Equal(t, "scott", args[0])
	}

	err = sb.Exec(context.Background())

	if assert.NoError(t, err, "query error") {
		if assert.Len(t, results, 1) {
			assert.Equal(t, "scott", results[0].Name)
			assert.Equal(t, "1000000", results[0].HungerLevel)
		}
	}
}
