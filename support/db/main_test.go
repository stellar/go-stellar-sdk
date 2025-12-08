package db

import (
	"testing"
	"time"

	"github.com/guregu/null"
	"github.com/stellar/go-stellar-sdk/support/db/dbtest"
	"github.com/stretchr/testify/assert"
)

type person struct {
	Name             string      `db:"name"`
	HungerLevel      string      `db:"hunger_level"`
	JsonValue        null.String `db:"json_value"`
	SomethingIgnored int         `db:"-"`
}

func TestGetTable(t *testing.T) {
	db := dbtest.Postgres(t).Load(testSchema)
	defer db.Close()
	sess := &Session{DB: db.Open()}
	defer sess.DB.Close()

	tbl := sess.GetTable("users")
	if assert.NotNil(t, tbl) {
		assert.Equal(t, "users", tbl.Name)
		assert.Equal(t, sess, tbl.Session)
	}

}

func TestAugmentDSN(t *testing.T) {
	configs := []ClientConfig{
		IdleTransactionTimeout(2 * time.Second),
		StatementTimeout(4 * time.Millisecond),
	}
	for _, testCase := range []struct {
		input    string
		expected string
	}{
		{"postgresql://localhost", "postgresql://localhost?idle_in_transaction_session_timeout=2000&statement_timeout=4"},
		{"postgresql://localhost/mydb?user=other&password=secret", "postgresql://localhost/mydb?idle_in_transaction_session_timeout=2000&password=secret&statement_timeout=4&user=other"},
		{"postgresql://localhost/mydb?user=other&idle_in_transaction_session_timeout=500", "postgresql://localhost/mydb?idle_in_transaction_session_timeout=500&statement_timeout=4&user=other"},
		{"host=localhost user=bob password=secret", "host=localhost user=bob password=secret idle_in_transaction_session_timeout=2000 statement_timeout=4"},
		{"host=localhost user=bob password=secret statement_timeout=32", "host=localhost user=bob password=secret statement_timeout=32 idle_in_transaction_session_timeout=2000"},
	} {
		t.Run(testCase.input, func(t *testing.T) {
			output := augmentDSN(testCase.input, configs)
			if output != testCase.expected {
				t.Fatalf("got %v but expected %v", output, testCase.expected)
			}
		})
	}
}
