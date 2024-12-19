// 2012 Go-MySQL-Driver Yazarlarının Telif Hakkı Saklıdır.
//
// Bu Kaynak Kod Formu, Mozilla Genel Kamu Lisansı, sürüm 2.0 şartlarına tabidir.
// Bu dosya ile birlikte MPL'nin bir kopyası dağıtılmadıysa,
// http://mozilla.org/MPL/2.0/ adresinden edinebilirsiniz.

// mysql paketi, Go'nun database/sql paketi için bir MySQL sürücüsü sağlar.
//
// Sürücü, database/sql paketi aracılığıyla kullanılmalıdır:
//
//	import "database/sql"
//	import _ "github.com/go-sql-driver/mysql"
//
//	db, err := sql.Open("mysql", "user:password@/dbname")
//
// Detaylar için https://github.com/go-sql-driver/mysql#usage adresine bakın
package mysql

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"net"
	"sync"
)

// MySQLDriver doğrudan erişilebilir hale getirmek için dışa aktarılmıştır.
// Genel olarak sürücü, database/sql paketi aracılığıyla kullanılır.
type MySQLDriver struct{}

// DialFunc, ağ bağlantısını kurmak için kullanılabilecek bir işlevdir.
// Özel arama işlevleri RegisterDial ile kaydedilmelidir.
//
// Kullanımdan kaldırıldı: kullanıcılar yerine DialContextFunc kaydetmelidir
type DialFunc func(addr string) (net.Conn, error)

// DialContextFunc, ağ bağlantısını kurmak için kullanılabilecek bir işlevdir.
// Özel arama işlevleri RegisterDialContext ile kaydedilmelidir
type DialContextFunc func(ctx context.Context, addr string) (net.Conn, error)

var (
	dialsLock sync.RWMutex
	dials     map[string]DialContextFunc
)

// RegisterDialContext, özel bir arama işlevi kaydeder. Daha sonra mynet(addr) ağ adresi ile kullanılabilir,
// burada mynet, kaydedilen yeni ağdır. Bağlantı için geçerli bağlam ve adres arama işlevine geçirilir.
func RegisterDialContext(net string, dial DialContextFunc) {
	dialsLock.Lock()
	defer dialsLock.Unlock()
	if dials == nil {
		dials = make(map[string]DialContextFunc)
	}
	dials[net] = dial
}

// DeregisterDialContext, verilen ağ ile kaydedilen özel arama işlevini kaldırır.
func DeregisterDialContext(net string) {
	dialsLock.Lock()
	defer dialsLock.Unlock()
	if dials != nil {
		delete(dials, net)
	}
}

// RegisterDial, özel bir arama işlevi kaydeder. Daha sonra mynet(addr) ağ adresi ile kullanılabilir,
// burada mynet, kaydedilen yeni ağdır. addr, arama işlevine parametre olarak geçirilir.
//
// Kullanımdan kaldırıldı: kullanıcılar yerine RegisterDialContext çağırmalıdır
func RegisterDial(network string, dial DialFunc) {
	RegisterDialContext(network, func(_ context.Context, addr string) (net.Conn, error) {
		return dial(addr)
	})
}

// Yeni Bağlantı Aç.
// DSN dizesinin nasıl biçimlendirildiği hakkında bilgi için https://github.com/go-sql-driver/mysql#dsn-data-source-name adresine bakın
func (d MySQLDriver) Open(dsn string) (driver.Conn, error) {
	cfg, err := ParseDSN(dsn)
	if err != nil {
		return nil, err
	}
	c := newConnector(cfg)
	return c.Connect(context.Background())
}

// Bu değişken -ldflags ile aşağıdaki gibi değiştirilebilir:
// go build "-ldflags=-X github.com/go-sql-driver/mysql.driverName=custom"
var driverName = "mysql"

func init() {
	if driverName != "" {
		sql.Register(driverName, &MySQLDriver{})
	}
}

// NewConnector yeni driver.Connector döndürür.
func NewConnector(cfg *Config) (driver.Connector, error) {
	cfg = cfg.Clone()
	// cfg'nin içeriğini normalize et, böylece NewConnector çağrıları MySQLDriver.OpenConnector ile aynı davranışa sahip olur
	if err := cfg.normalize(); err != nil {
		return nil, err
	}
	return newConnector(cfg), nil
}

// OpenConnector driver.DriverContext'i uygular.
func (d MySQLDriver) OpenConnector(dsn string) (driver.Connector, error) {
	cfg, err := ParseDSN(dsn)
	if err != nil {
		return nil, err
	}
	return newConnector(cfg), nil
}
