package db

import (
	"context"
	"reflect"
	"sort"

	"github.com/lib/pq"

	"github.com/stellar/go-stellar-sdk/support/errors"
)

// ErrSealed is returned when trying to add rows to the FastBatchInsertBuilder after Exec() is called.
// Once Exec() is called no more rows can be added to the FastBatchInsertBuilder unless you call Reset()
// which clears out the old rows from the FastBatchInsertBuilder.
var ErrSealed = errors.New("cannot add more rows after Exec() without calling Reset() first")

// ErrNoTx is returned when Exec() is called outside of a transaction.
var ErrNoTx = errors.New("cannot call Exec() outside of a transaction")

// FastBatchInsertBuilder works like sq.InsertBuilder but has a better support for batching
// large number of rows.
// It is NOT safe for concurrent use.
// It does NOT support updating existing rows.
type FastBatchInsertBuilder struct {
	columns       []string
	rows          [][]interface{}
	rowStructType reflect.Type
	sealed        bool
}

// Row adds a new row to the batch. All rows must have exactly the same columns
// (map keys). Otherwise, error will be returned. Please note that rows are not
// added one by one but in batches when `Exec` is called.
func (b *FastBatchInsertBuilder) Row(row map[string]interface{}) error {
	if b.sealed {
		return ErrSealed
	}

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

	return nil
}

// RowStruct adds a new row to the batch. All rows must have exactly the same columns
// (map keys). Otherwise, error will be returned. Please note that rows are not
// added one by one but in batches when `Exec` is called.
func (b *FastBatchInsertBuilder) RowStruct(row interface{}) error {
	if b.sealed {
		return ErrSealed
	}

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

	return nil
}

// Len returns the number of rows held in memory by the FastBatchInsertBuilder.
func (b *FastBatchInsertBuilder) Len() int {
	return len(b.rows)
}

// Exec inserts rows in a single COPY statement. Once Exec is called no more rows
// can be added to the FastBatchInsertBuilder unless Reset is called.
// Exec must be called within a transaction.
func (b *FastBatchInsertBuilder) Exec(ctx context.Context, session SessionInterface, tableName string) error {
	b.sealed = true
	if session.GetTx() == nil {
		return ErrNoTx
	}

	if len(b.rows) == 0 {
		return nil
	}

	tx := session.GetTx()
	stmt, err := tx.PrepareContext(ctx, pq.CopyIn(tableName, b.columns...))
	if err != nil {
		return err
	}

	for _, row := range b.rows {
		if _, err = stmt.ExecContext(ctx, row...); err != nil {
			// we need to close the statement otherwise the session
			// will always return bad connection errors when executing
			// any other sql statements,
			// see https://github.com/stellar/go-stellar-sdk/pull/316#issuecomment-368990324
			stmt.Close()
			return err
		}
	}

	if err = stmt.Close(); err != nil {
		return err
	}
	return nil
}

// Reset clears out all the rows contained in the FastBatchInsertBuilder.
// After Reset is called new rows can be added to the FastBatchInsertBuilder.
func (b *FastBatchInsertBuilder) Reset() {
	b.sealed = false
	b.columns = nil
	b.rows = nil
	b.rowStructType = nil
}
