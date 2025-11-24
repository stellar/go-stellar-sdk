package db

import (
	"context"
	"database/sql"
	"reflect"

	"github.com/stellar/go-stellar-sdk/support/errors"
)

// Exec executes the query that has been previously configured on the
// UpdateBuilder.
func (ub *UpdateBuilder) Exec(ctx context.Context) (sql.Result, error) {
	r, err := ub.Table.Session.Exec(ctx, ub.sql)
	if err != nil {
		return nil, errors.Wrap(err, "select failed")
	}

	return r, nil
}

// Limit is a passthrough call to the squirrel.  See
// https://godoc.org/github.com/Masterminds/squirrel#UpdateBuilder.Limit
func (ub *UpdateBuilder) Limit(limit uint64) *UpdateBuilder {
	ub.sql = ub.sql.Limit(limit)
	return ub
}

// Offset is a passthrough call to the squirrel.  See
// https://godoc.org/github.com/Masterminds/squirrel#UpdateBuilder.Offset
func (ub *UpdateBuilder) Offset(offset uint64) *UpdateBuilder {
	ub.sql = ub.sql.Offset(offset)
	return ub
}

// OrderBy is a passthrough call to the squirrel.  See
// https://godoc.org/github.com/Masterminds/squirrel#UpdateBuilder.OrderBy
func (ub *UpdateBuilder) OrderBy(
	orderBys ...string,
) *UpdateBuilder {
	ub.sql = ub.sql.OrderBy(orderBys...)
	return ub
}

// Prefix is a passthrough call to the squirrel.  See
// https://godoc.org/github.com/Masterminds/squirrel#UpdateBuilder.Prefix
func (ub *UpdateBuilder) Prefix(
	sql string,
	args ...interface{},
) *UpdateBuilder {
	ub.sql = ub.sql.Prefix(sql, args...)
	return ub
}

// Set is a passthrough call to the squirrel.  See
// https://godoc.org/github.com/Masterminds/squirrel#UpdateBuilder.Set
func (ub *UpdateBuilder) Set(column string, value interface{}) *UpdateBuilder {
	ub.sql = ub.sql.Set(column, value)
	return ub
}

// SetMap is a passthrough call to the squirrel.  See
// https://godoc.org/github.com/Masterminds/squirrel#UpdateBuilder.SetMap
func (ub *UpdateBuilder) SetMap(clauses map[string]interface{}) *UpdateBuilder {
	ub.sql = ub.sql.SetMap(clauses)
	return ub
}

// SetStruct is using `db` tag on the struct and updates the query with struct
// values for each field (except `ignored` fields).
func (ub *UpdateBuilder) SetStruct(s interface{}, ignored []string) *UpdateBuilder {
	ignoredMap := map[string]bool{}
	for _, ig := range ignored {
		ignoredMap[ig] = true
	}

	cols := ColumnsForStruct(s)
	row := reflect.ValueOf(s)
	rvals := mapper.FieldsByName(row, cols)

	for i, col := range cols {
		if ignoredMap[col] {
			continue
		}
		ub.Set(col, rvals[i].Interface())
	}

	return ub
}

// Suffix is a passthrough call to the squirrel.  See
// https://godoc.org/github.com/Masterminds/squirrel#UpdateBuilder.Suffix
func (ub *UpdateBuilder) Suffix(
	sql string,
	args ...interface{},
) *UpdateBuilder {
	ub.sql = ub.sql.Suffix(sql, args...)
	return ub
}

// Where is a passthrough call to the squirrel.  See
// https://godoc.org/github.com/Masterminds/squirrel#UpdateBuilder.Where
func (ub *UpdateBuilder) Where(
	pred interface{},
	args ...interface{},
) *UpdateBuilder {
	ub.sql = ub.sql.Where(pred, args...)
	return ub
}
