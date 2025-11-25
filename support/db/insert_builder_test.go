package db

import (
	"context"
	"testing"

	"github.com/stellar/go-stellar-sdk/support/db/dbtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInsertBuilder_Exec(t *testing.T) {
	ctx := context.Background()
	db := dbtest.Postgres(t).Load(testSchema)
	defer db.Close()
	sess := &Session{DB: db.Open()}
	defer sess.DB.Close()

	tbl := sess.GetTable("people")

	_, err := tbl.Insert(person{
		Name:        "bubba",
		HungerLevel: "120",
	}).Exec(ctx)

	if assert.NoError(t, err) {
		var found []person
		err = sess.SelectRaw(ctx,
			&found,
			"SELECT * FROM people WHERE name = ?",
			"bubba",
		)

		require.NoError(t, err)

		if assert.Len(t, found, 1) {
			assert.Equal(t, "bubba", found[0].Name)
			assert.Equal(t, "120", found[0].HungerLevel)
		}
	}

	// no rows
	_, err = tbl.Insert().Exec(ctx)
	if assert.Error(t, err) {
		assert.IsType(t, &NoRowsError{}, err)
		assert.EqualError(t, err, "no rows provided to insert")
	}

	// multi rows
	r, err := tbl.Insert(person{
		Name:        "bubba2",
		HungerLevel: "120",
	}, person{
		Name:        "bubba3",
		HungerLevel: "120",
	}).Exec(ctx)

	if assert.NoError(t, err) {
		count, err2 := r.RowsAffected()
		require.NoError(t, err2)
		assert.Equal(t, int64(2), count)
	}

	// invalid columns in struct
	_, err = tbl.Insert(struct {
		Name        string `db:"name"`
		HungerLevel string `db:"hunger_level"`
		NotAColumn  int    `db:"not_a_column"`
	}{
		Name:        "bubba2",
		HungerLevel: "120",
		NotAColumn:  3,
	}).Exec(ctx)

	assert.Error(t, err)
}
