package db

import (
	"context"

	"github.com/stellar/go-stellar-sdk/support/errors"
)

// Exec executes the query represented by the builder, populating the
// destination with the results returned by running the query against the
// current database session.
func (gb *GetBuilder) Exec(ctx context.Context) error {
	err := gb.Table.Session.Get(ctx, gb.dest, gb.sql)
	if err != nil {
		return errors.Wrap(err, "get failed")
	}

	return nil
}

// Offset is a passthrough call to the squirrel.  See
// https://godoc.org/github.com/Masterminds/squirrel#SelectBuilder.Offset
func (gb *GetBuilder) Offset(offset uint64) *GetBuilder {
	gb.sql = gb.sql.Offset(offset)
	return gb
}

// OrderBy is a passthrough call to the squirrel.  See
// https://godoc.org/github.com/Masterminds/squirrel#SelectBuilder.OrderBy
func (gb *GetBuilder) OrderBy(
	orderBys ...string,
) *GetBuilder {
	gb.sql = gb.sql.OrderBy(orderBys...)
	return gb
}

// Prefix is a passthrough call to the squirrel.  See
// https://godoc.org/github.com/Masterminds/squirrel#SelectBuilder.Prefix
func (gb *GetBuilder) Prefix(
	sql string,
	args ...interface{},
) *GetBuilder {
	gb.sql = gb.sql.Prefix(sql, args...)
	return gb
}

// Suffix is a passthrough call to the squirrel.  See
// https://godoc.org/github.com/Masterminds/squirrel#SelectBuilder.Suffix
func (gb *GetBuilder) Suffix(
	sql string,
	args ...interface{},
) *GetBuilder {
	gb.sql = gb.sql.Suffix(sql, args...)
	return gb
}

// Where is a passthrough call to the squirrel.  See
// https://godoc.org/github.com/Masterminds/squirrel#GetBuilder.Where
func (gb *GetBuilder) Where(
	pred interface{},
	args ...interface{},
) *GetBuilder {
	gb.sql = gb.sql.Where(pred, args...)
	return gb
}
