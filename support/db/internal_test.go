package db

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

const testSchema = `
CREATE TABLE  IF NOT EXISTS people (
    name character varying NOT NULL,
    hunger_level integer NOT NULL,
    json_value jsonb,
    PRIMARY KEY (name)
);
DELETE FROM people;
INSERT INTO people (name, hunger_level) VALUES ('scott', 1000000);
INSERT INTO people (name, hunger_level) VALUES ('jed', 10);
INSERT INTO people (name, hunger_level) VALUES ('bartek', 10);
`

func TestColumnsForStruct(t *testing.T) {
	cases := []struct {
		Name     string
		Struct   interface{}
		Expected []string
	}{
		{
			Name: "simple",
			Struct: struct {
				Name string `db:"name"`
			}{},
			Expected: []string{"name"},
		},
		{
			Name: "simple pointer",
			Struct: &struct {
				Name string `db:"name"`
			}{},
			Expected: []string{"name"},
		},
		{
			Name: "slice",
			Struct: []struct {
				Name string `db:"name"`
			}{},
			Expected: []string{"name"},
		},
		{
			Name: "slice pointer",
			Struct: &[]struct {
				Name string `db:"name"`
			}{},
			Expected: []string{"name"},
		},
		{
			Name: "ignored",
			Struct: struct {
				Name   string `db:"name"`
				Ignore string `db:"-"`
				Age    string `db:"age"`
			}{},
			Expected: []string{"age", "name"},
		},
		{
			Name: "unannotated",
			Struct: struct {
				Name  string `db:"name"`
				Age   string
				Level int `json:"level"`
			}{},
			Expected: []string{"Age", "Level", "name"},
		},
		{
			Name: "private",
			Struct: struct {
				Name string `db:"name"`
				age  string
			}{},
			Expected: []string{"name"},
		},
	}

	for _, kase := range cases {
		actual := ColumnsForStruct(kase.Struct)

		assert.Equal(t, kase.Expected, actual, "case '%s' failed", kase.Name)
	}
}
