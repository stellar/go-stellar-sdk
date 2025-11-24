package db

import (
	"context"
	"testing"

	"github.com/stellar/go-stellar-sdk/support/db/dbtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeleteBuilder_Exec(t *testing.T) {
	db := dbtest.Postgres(t).Load(testSchema)
	defer db.Close()
	sess := &Session{DB: db.Open()}
	defer sess.DB.Close()

	ctx := context.Background()
	tbl := sess.GetTable("people")
	r, err := tbl.Delete("name = ?", "scott").Exec(ctx)

	if assert.NoError(t, err, "query error") {
		actual, err := r.RowsAffected()
		require.NoError(t, err)
		assert.Equal(t, int64(1), actual)

		var found int
		err = sess.GetRaw(ctx, &found, "SELECT COUNT(*) FROM people WHERE name = ?", "scott")
		require.NoError(t, err)
		assert.Equal(t, 0, found)
	}
}
