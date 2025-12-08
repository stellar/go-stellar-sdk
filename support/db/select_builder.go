package db

import (
	"context"

	"github.com/stellar/go-stellar-sdk/support/errors"
)

// Exec executes the query represented by the builder, populating the
// destination with the results returned by running the query against the
// current database session.
func (sb *SelectBuilder) Exec(ctx context.Context) error {
	err := sb.Table.Session.Select(ctx, sb.dest, sb.sql)
	if err != nil {
		return errors.Wrap(err, "select failed")
	}

	return nil
}

// Limit is a passthrough call to the squirrel.  See
// https://godoc.org/github.com/Masterminds/squirrel#SelectBuilder.Limit
func (sb *SelectBuilder) Limit(limit uint64) *SelectBuilder {
	sb.sql = sb.sql.Limit(limit)
	return sb
}

// Offset is a passthrough call to the squirrel.  See
// https://godoc.org/github.com/Masterminds/squirrel#SelectBuilder.Offset
func (sb *SelectBuilder) Offset(offset uint64) *SelectBuilder {
	sb.sql = sb.sql.Offset(offset)
	return sb
}

// OrderBy is a passthrough call to the squirrel.  See
// https://godoc.org/github.com/Masterminds/squirrel#SelectBuilder.OrderBy
func (sb *SelectBuilder) OrderBy(
	orderBys ...string,
) *SelectBuilder {
	sb.sql = sb.sql.OrderBy(orderBys...)
	return sb
}

// Prefix is a passthrough call to the squirrel.  See
// https://godoc.org/github.com/Masterminds/squirrel#SelectBuilder.Prefix
func (sb *SelectBuilder) Prefix(
	sql string,
	args ...interface{},
) *SelectBuilder {
	sb.sql = sb.sql.Prefix(sql, args...)
	return sb
}

// Suffix is a passthrough call to the squirrel.  See
// https://godoc.org/github.com/Masterminds/squirrel#SelectBuilder.Suffix
func (sb *SelectBuilder) Suffix(
	sql string,
	args ...interface{},
) *SelectBuilder {
	sb.sql = sb.sql.Suffix(sql, args...)
	return sb
}

// Where is a passthrough call to the squirrel.  See
// https://godoc.org/github.com/Masterminds/squirrel#SelectBuilder.Where
func (sb *SelectBuilder) Where(
	pred interface{},
	args ...interface{},
) *SelectBuilder {
	sb.sql = sb.sql.Where(pred, args...)
	return sb
}
