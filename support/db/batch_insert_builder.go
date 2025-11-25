package db

import (
	"context"
	"fmt"
	"reflect"
	"sort"

	sq "github.com/Masterminds/squirrel"
	"github.com/stellar/go-stellar-sdk/support/errors"
)

// BatchInsertBuilder works like sq.InsertBuilder but has a better support for batching
// large number of rows.
// It is NOT safe for concurrent use.
type BatchInsertBuilder struct {
	Table *Table
	// MaxBatchSize defines the maximum size of a batch. If this number is
	// reached after calling Row() it will call Exec() immediately inserting
	// all rows to a DB.
	// Zero (default) will not add rows until explicitly calling Exec.
	MaxBatchSize int

	// Suffix adds a sql expression to the end of the query (e.g. an ON CONFLICT clause)
	Suffix string

	columns       []string
	rows          [][]interface{}
	rowStructType reflect.Type
}

// Row adds a new row to the batch. All rows must have exactly the same columns
// (map keys). Otherwise, error will be returned. Please note that rows are not
// added one by one but in batches when `Exec` is called (or `MaxBatchSize` is
// reached).
func (b *BatchInsertBuilder) Row(ctx context.Context, row map[string]interface{}) error {
	if b.columns == nil {
		b.columns = make([]string, 0, len(row))
		b.rows = make([][]interface{}, 0)

		for column := range row {
			b.columns = append(b.columns, column)
		}

		sort.Strings(b.columns)
	}

	if len(b.columns) != len(row) {
		return errors.Errorf("invalid number of columns (expected=%d, actual=%d)", len(b.columns), len(row))
	}

	rowSlice := make([]interface{}, 0, len(b.columns))
	for _, column := range b.columns {
		val, ok := row[column]
		if !ok {
			return errors.Errorf(`column "%s" does not exist`, column)
		}
		rowSlice = append(rowSlice, val)
	}

	b.rows = append(b.rows, rowSlice)

	// Call Exec when MaxBatchSize is reached.
	if len(b.rows) == b.MaxBatchSize {
		return b.Exec(ctx)
	}

	return nil
}

func (b *BatchInsertBuilder) RowStruct(ctx context.Context, row interface{}) error {
	if b.columns == nil {
		b.columns = ColumnsForStruct(row)
		b.rows = make([][]interface{}, 0)
	}

	rowType := reflect.TypeOf(row)
	if b.rowStructType == nil {
		b.rowStructType = rowType
	} else if b.rowStructType != rowType {
		return errors.Errorf(`expected value of type "%s" but got "%s" value`, b.rowStructType.String(), rowType.String())
	}

	rrow := reflect.ValueOf(row)
	rvals := mapper.FieldsByName(rrow, b.columns)

	// convert fields values to interface{}
	columnValues := make([]interface{}, len(b.columns))
	for i, rval := range rvals {
		columnValues[i] = rval.Interface()
	}

	b.rows = append(b.rows, columnValues)

	// Call Exec when MaxBatchSize is reached.
	if len(b.rows) == b.MaxBatchSize {
		return b.Exec(ctx)
	}

	return nil
}

func (b *BatchInsertBuilder) insertSQL() sq.InsertBuilder {
	insertStatement := sq.Insert(b.Table.Name).Columns(b.columns...)
	if len(b.Suffix) > 0 {
		return insertStatement.Suffix(b.Suffix)
	}
	return insertStatement
}

// Exec inserts rows in batches. In case of errors it's possible that some batches
// were added so this should be run in a DB transaction for easy rollbacks.
func (b *BatchInsertBuilder) Exec(ctx context.Context) error {
	sql := b.insertSQL()
	paramsCount := 0

	for _, row := range b.rows {
		sql = sql.Values(row...)
		paramsCount += len(row)

		if paramsCount > PostgresQueryMaxParams-2*len(b.columns) {
			_, err := b.Table.Session.Exec(ctx, sql)
			if err != nil {
				return errors.Wrap(err, fmt.Sprintf("error adding values while inserting to %s", b.Table.Name))
			}
			paramsCount = 0
			sql = b.insertSQL()
		}
	}

	// Insert last batch
	if paramsCount > 0 {
		_, err := b.Table.Session.Exec(ctx, sql)
		if err != nil {
			return errors.Wrap(err, fmt.Sprintf("error adding values while inserting to %s", b.Table.Name))
		}
	}

	// Clear the rows so user can reuse it for batch inserting to a single table
	b.rows = make([][]interface{}, 0)
	return nil
}
