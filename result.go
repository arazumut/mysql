// Go MySQL Sürücüsü - Go'nun database/sql paketi için bir MySQL Sürücüsü
//
// 2012 Go-MySQL-Driver Yazarlarının Tüm Hakları Saklıdır.
//
// Bu Kaynak Kod Formu, Mozilla Genel Kamu Lisansı, sürüm 2.0 şartlarına tabidir.
// MPL'nin bir kopyası bu dosya ile dağıtılmadıysa, http://mozilla.org/MPL/2.0/ adresinden edinebilirsiniz.

package mysql

import "database/sql/driver"

// Result, *connection.Result üzerinden erişilemeyen verileri ortaya çıkarır.
//
// Bu, sql.Conn.Raw() kullanılarak ve döndürülen sonucun aşağıya dökümü yapılarak erişilebilir:
//
//	res, err := rawConn.Exec(...)
//	res.(mysql.Result).AllRowsAffected()
type Result interface {
	driver.Result
	// AllRowsAffected, her yürütülen ifade için etkilenen satırları içeren bir dilim döndürür.
	AllRowsAffected() []int64
	// AllLastInsertIds, her yürütülen ifade için son eklenen kimliği içeren bir dilim döndürür.
	AllLastInsertIds() []int64
}

type mysqlResult struct {
	// Her yürütülen ifade sonucu için her iki dilimde bir giriş oluşturulur.
	affectedRows []int64
	insertIds    []int64
}

func (res *mysqlResult) LastInsertId() (int64, error) {
	if len(res.insertIds) == 0 {
		return 0, nil
	}
	return res.insertIds[len(res.insertIds)-1], nil
}

func (res *mysqlResult) RowsAffected() (int64, error) {
	if len(res.affectedRows) == 0 {
		return 0, nil
	}
	return res.affectedRows[len(res.affectedRows)-1], nil
}

func (res *mysqlResult) AllLastInsertIds() []int64 {
	return append([]int64{}, res.insertIds...)
}

func (res *mysqlResult) AllRowsAffected() []int64 {
	return append([]int64{}, res.affectedRows...)
}
