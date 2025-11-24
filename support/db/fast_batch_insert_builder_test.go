package db

import (
	"context"
	"testing"

	"github.com/guregu/null"
	"github.com/stretchr/testify/assert"

	"github.com/stellar/go-stellar-sdk/support/db/dbtest"
)

func TestFastBatchInsertBuilder(t *testing.T) {
	db := dbtest.Postgres(t).Load(testSchema)
	defer db.Close()
	sess := &Session{DB: db.Open()}
	defer sess.DB.Close()

	insertBuilder := &FastBatchInsertBuilder{}

	assert.NoError(t,
		insertBuilder.Row(map[string]interface{}{
			"name":         "bubba",
			"hunger_level": "1",
			"json_value":   `{"bump_to": "97"}`,
		}),
	)

	assert.EqualError(t,
		insertBuilder.Row(map[string]interface{}{
			"name": "bubba",
		}),
		"invalid number of columns (expected=3, actual=1)",
	)

	assert.EqualError(t,
		insertBuilder.Row(map[string]interface{}{
			"name":       "bubba",
			"city":       "London",
			"json_value": `{"bump_to": "98"}`,
		}),
		"column \"hunger_level\" does not exist",
	)

	assert.NoError(t,
		insertBuilder.RowStruct(hungerRow{
			Name:        "bubba2",
			HungerLevel: "9",
			JsonValue:   `{"bump_to": "98"}`,
		}),
	)

	assert.EqualError(t,
		insertBuilder.RowStruct(invalidHungerRow{
			Name:        "bubba",
			HungerLevel: "2",
			LastName:    "b",
		}),
		"expected value of type \"db.hungerRow\" but got \"db.invalidHungerRow\" value",
	)
	assert.Equal(t, 2, insertBuilder.Len())
	assert.Equal(t, false, insertBuilder.sealed)

	ctx := context.Background()
	assert.EqualError(t,
		insertBuilder.Exec(ctx, sess, "people"),
		"cannot call Exec() outside of a transaction",
	)
	assert.Equal(t, true, insertBuilder.sealed)

	assert.NoError(t, sess.Begin(ctx))
	assert.NoError(t, insertBuilder.Exec(ctx, sess, "people"))
	assert.Equal(t, 2, insertBuilder.Len())
	assert.Equal(t, true, insertBuilder.sealed)

	var found []person
	assert.NoError(t, sess.SelectRaw(context.Background(), &found, `SELECT * FROM people WHERE name like 'bubba%'`))
	assert.Equal(
		t,
		found,
		[]person{
			{Name: "bubba", HungerLevel: "1", JsonValue: null.NewString(`{"bump_to": "97"}`, true)},
			{Name: "bubba2", HungerLevel: "9", JsonValue: null.NewString(`{"bump_to": "98"}`, true)},
		},
	)

	assert.EqualError(t,
		insertBuilder.Row(map[string]interface{}{
			"name":         "bubba3",
			"hunger_level": "100",
		}),
		"cannot add more rows after Exec() without calling Reset() first",
	)
	assert.Equal(t, 2, insertBuilder.Len())
	assert.Equal(t, true, insertBuilder.sealed)

	insertBuilder.Reset()
	assert.Equal(t, 0, insertBuilder.Len())
	assert.Equal(t, false, insertBuilder.sealed)

	assert.NoError(t,
		insertBuilder.Row(map[string]interface{}{
			"name":         "bubba3",
			"hunger_level": "3",
		}),
	)
	assert.Equal(t, 1, insertBuilder.Len())
	assert.Equal(t, false, insertBuilder.sealed)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	assert.EqualError(t,
		insertBuilder.Exec(ctx, sess, "people"),
		"context canceled",
	)
	assert.Equal(t, 1, insertBuilder.Len())
	assert.Equal(t, true, insertBuilder.sealed)

	assert.NoError(t, sess.SelectRaw(context.Background(), &found, `SELECT * FROM people WHERE name like 'bubba%'`))
	assert.Equal(
		t,
		found,
		[]person{
			{Name: "bubba", HungerLevel: "1", JsonValue: null.NewString(`{"bump_to": "97"}`, true)},
			{Name: "bubba2", HungerLevel: "9", JsonValue: null.NewString(`{"bump_to": "98"}`, true)},
		},
	)
	assert.NoError(t, sess.Rollback())

	assert.NoError(t, sess.SelectRaw(context.Background(), &found, `SELECT * FROM people WHERE name like 'bubba%'`))
	assert.Empty(t, found)
}
