// Go MySQL Sürücüsü - Go'nun database/sql paketi için bir MySQL Sürücüsü
//
// Telif Hakkı 2012 Go-MySQL-Driver Yazarlarına aittir. Tüm hakları saklıdır.
//
// Bu Kaynak Kod Formu, Mozilla Genel Kamu Lisansı, sürüm 2.0 şartlarına tabidir.
// Bu dosya ile birlikte MPL'nin bir kopyası dağıtılmadıysa, http://mozilla.org/MPL/2.0/ adresinden edinebilirsiniz.

package mysql

import (
	"database/sql/driver"
	"io"
	"math"
	"reflect"
)

type resultSet struct {
	columns     []mysqlField
	columnNames []string
	done        bool
}

type mysqlRows struct {
	mc     *mysqlConn
	rs     resultSet
	finish func()
}

type binaryRows struct {
	mysqlRows
}

type textRows struct {
	mysqlRows
}

func (rows *mysqlRows) Columns() []string {
	if rows.rs.columnNames != nil {
		return rows.rs.columnNames
	}

	columns := make([]string, len(rows.rs.columns))
	if rows.mc != nil && rows.mc.cfg.ColumnsWithAlias {
		for i := range columns {
			if tableName := rows.rs.columns[i].tableName; len(tableName) > 0 {
				columns[i] = tableName + "." + rows.rs.columns[i].name
			} else {
				columns[i] = rows.rs.columns[i].name
			}
		}
	} else {
		for i := range columns {
			columns[i] = rows.rs.columns[i].name
		}
	}

	rows.rs.columnNames = columns
	return columns
}

func (rows *mysqlRows) ColumnTypeDatabaseTypeName(i int) string {
	return rows.rs.columns[i].typeDatabaseName()
}

func (rows *mysqlRows) ColumnTypeNullable(i int) (nullable, ok bool) {
	return rows.rs.columns[i].flags&flagNotNULL == 0, true
}

func (rows *mysqlRows) ColumnTypePrecisionScale(i int) (int64, int64, bool) {
	column := rows.rs.columns[i]
	decimals := int64(column.decimals)

	switch column.fieldType {
	case fieldTypeDecimal, fieldTypeNewDecimal:
		if decimals > 0 {
			return int64(column.length) - 2, decimals, true
		}
		return int64(column.length) - 1, decimals, true
	case fieldTypeTimestamp, fieldTypeDateTime, fieldTypeTime:
		return decimals, decimals, true
	case fieldTypeFloat, fieldTypeDouble:
		if decimals == 0x1f {
			return math.MaxInt64, math.MaxInt64, true
		}
		return math.MaxInt64, decimals, true
	}

	return 0, 0, false
}

func (rows *mysqlRows) ColumnTypeScanType(i int) reflect.Type {
	return rows.rs.columns[i].scanType()
}

func (rows *mysqlRows) Close() (err error) {
	if f := rows.finish; f != nil {
		f()
		rows.finish = nil
	}

	mc := rows.mc
	if mc == nil {
		return nil
	}
	if err := mc.error(); err != nil {
		return err
	}

	// Okunmamış paketleri akıştan çıkar
	if !rows.rs.done {
		err = mc.readUntilEOF()
	}
	if err == nil {
		handleOk := mc.clearResult()
		if err = handleOk.discardResults(); err != nil {
			return err
		}
	}

	rows.mc = nil
	return err
}

func (rows *mysqlRows) HasNextResultSet() bool {
	if rows.mc == nil {
		return false
	}
	return rows.mc.status&statusMoreResultsExists != 0
}

func (rows *mysqlRows) nextResultSet() (int, error) {
	if rows.mc == nil {
		return 0, io.EOF
	}
	if err := rows.mc.error(); err != nil {
		return 0, err
	}

	// Okunmamış paketleri akıştan çıkar
	if !rows.rs.done {
		if err := rows.mc.readUntilEOF(); err != nil {
			return 0, err
		}
		rows.rs.done = true
	}

	if !rows.HasNextResultSet() {
		rows.mc = nil
		return 0, io.EOF
	}
	rows.rs = resultSet{}
	// rows.mc.affectedRows ve rows.mc.insertIds her nextResultSet çağrısında birikir.
	resLen, err := rows.mc.resultUnchanged().readResultSetHeaderPacket()
	if err != nil {
		// Çoklu sonuçlar bayrağı hakkında temizleme yap
		rows.rs.done = true
		rows.mc.status = rows.mc.status & (^statusMoreResultsExists)
	}
	return resLen, err
}

func (rows *mysqlRows) nextNotEmptyResultSet() (int, error) {
	for {
		resLen, err := rows.nextResultSet()
		if err != nil {
			return 0, err
		}

		if resLen > 0 {
			return resLen, nil
		}

		rows.rs.done = true
	}
}

func (rows *binaryRows) NextResultSet() error {
	resLen, err := rows.nextNotEmptyResultSet()
	if err != nil {
		return err
	}

	rows.rs.columns, err = rows.mc.readColumns(resLen)
	return err
}

func (rows *binaryRows) Next(dest []driver.Value) error {
	if mc := rows.mc; mc != nil {
		if err := mc.error(); err != nil {
			return err
		}

		// Akıştan bir sonraki satırı al
		return rows.readRow(dest)
	}
	return io.EOF
}

func (rows *textRows) NextResultSet() error {
	resLen, err := rows.nextNotEmptyResultSet()
	if err != nil {
		return err
	}

	rows.rs.columns, err = rows.mc.readColumns(resLen)
	return err
}

func (rows *textRows) Next(dest []driver.Value) error {
	if mc := rows.mc; mc != nil {
		if err := mc.error(); err != nil {
			return err
		}

		// Akıştan bir sonraki satırı al
		return rows.readRow(dest)
	}
	return io.EOF
}
