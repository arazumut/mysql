// Go MySQL Sürücüsü - Go'nun database/sql paketi için bir MySQL Sürücüsü
//
// Telif Hakkı 2013 Go-MySQL-Driver Yazarları. Tüm hakları saklıdır.
//
// Bu Kaynak Kod Formu, Mozilla Kamu Lisansı, sürüm 2.0 şartlarına tabidir.
// MPL'nin bir kopyası bu dosya ile birlikte dağıtılmadıysa,
// http://mozilla.org/MPL/2.0/ adresinden edinebilirsiniz.

//go:build linux || darwin || dragonfly || freebsd || netbsd || openbsd || solaris || illumos
// +build linux darwin dragonfly freebsd netbsd openbsd solaris illumos

package mysql

import (
	"testing"
	"time"
)

func TestStaleConnectionChecks(t *testing.T) {
	runTestsParallel(t, dsn, func(dbt *DBTest, _ string) {
		dbt.mustExec("SET @@SESSION.wait_timeout = 2")

		if err := dbt.db.Ping(); err != nil {
			dbt.Fatal(err)
		}

		// MySQL'in bağlantımızı kapatmasını bekleyin
		time.Sleep(3 * time.Second)

		tx, err := dbt.db.Begin()
		if err != nil {
			dbt.Fatal(err)
		}

		if err := tx.Rollback(); err != nil {
			dbt.Fatal(err)
		}
	})
}
