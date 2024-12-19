// Go MySQL Sürücüsü - Go'nun database/sql paketi için bir MySQL Sürücüsü
//
// Telif Hakkı 2019 Go-MySQL-Driver Yazarlarına aittir. Tüm hakları saklıdır.
//
// Bu Kaynak Kod Formu, Mozilla Kamu Lisansı, sürüm 2.0 şartlarına tabidir.
// MPL'nin bir kopyası bu dosya ile birlikte dağıtılmadıysa,
// http://mozilla.org/MPL/2.0/ adresinden edinebilirsiniz.

//go:build !linux && !darwin && !dragonfly && !freebsd && !netbsd && !openbsd && !solaris && !illumos
// +build !linux,!darwin,!dragonfly,!freebsd,!netbsd,!openbsd,!solaris,!illumos

package mysql

import "net"

func connCheck(conn net.Conn) error {
	return nil
}
