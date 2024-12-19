// Go MySQL Sürücüsü - Go'nun database/sql paketi için bir MySQL Sürücüsü
//
// Telif Hakkı 2013 Go-MySQL-Driver Yazarlarına aittir. Tüm hakları saklıdır.
//
// Bu Kaynak Kod Formu, Mozilla Genel Kamu Lisansı, sürüm 2.0 şartlarına tabidir.
// MPL'nin bir kopyası bu dosya ile dağıtılmadıysa, http://mozilla.org/MPL/2.0/ adresinden edinebilirsiniz.

package mysql

import (
	"bytes"
	"errors"
	"log"
	"testing"
)

func TestHatalarLoggerAyarla(t *testing.T) {
	onceki := defaultLogger
	defer func() {
		defaultLogger = onceki
	}()

	// logger kur
	const beklenen = "ön ek: test\n"
	buffer := bytes.NewBuffer(make([]byte, 0, 64))
	logger := log.New(buffer, "ön ek: ", 0)

	// yazdır
	SetLogger(logger)
	defaultLogger.Print("test")

	// sonucu kontrol et
	if gercek := buffer.String(); gercek != beklenen {
		t.Errorf("beklenen %q, alınan %q", beklenen, gercek)
	}
}

func TestHatalarStrictIgnoreNotes(t *testing.T) {
	runTests(t, dsn+"&sql_notes=false", func(dbt *DBTest) {
		dbt.mustExec("DROP TABLE IF EXISTS does_not_exist")
	})
}

func TestMySQLErrIs(t *testing.T) {
	altYapiHatasi := &MySQLError{Number: 1234, Message: "sunucu yanıyor"}
	digerAltYapiHatasi := &MySQLError{Number: 1234, Message: "veri merkezi su altında"}
	if !errors.Is(altYapiHatasi, digerAltYapiHatasi) {
		t.Errorf("hataların aynı olması bekleniyordu: %+v %+v", altYapiHatasi, digerAltYapiHatasi)
	}

	farkliKodHatasi := &MySQLError{Number: 5678, Message: "sunucu yanıyor"}
	if errors.Is(altYapiHatasi, farkliKodHatasi) {
		t.Fatalf("hataların farklı olması bekleniyordu: %+v %+v", altYapiHatasi, farkliKodHatasi)
	}

	mysqlOlmayanHata := errors.New("mysql hatası değil")
	if errors.Is(altYapiHatasi, mysqlOlmayanHata) {
		t.Fatalf("hataların farklı olması bekleniyordu: %+v %+v", altYapiHatasi, mysqlOlmayanHata)
	}
}
