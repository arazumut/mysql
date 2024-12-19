// Go MySQL Sürücüsü - Go'nun database/sql paketi için bir MySQL Sürücüsü
//
// Telif Hakkı 2018 Go-MySQL-Driver Yazarlarına aittir. Tüm hakları saklıdır.
//
// Bu Kaynak Kod Formu, Mozilla Public License, v. 2.0 şartlarına tabidir.
// Bu dosya ile birlikte MPL'nin bir kopyası dağıtılmadıysa,
// http://mozilla.org/MPL/2.0/ adresinden edinebilirsiniz.

package mysql

import (
	"context"
	"database/sql/driver"
	"net"
	"os"
	"strconv"
	"strings"
)

type connector struct {
	cfg               *Config // değiştirilemez özel kopya.
	encodedAttributes string  // Kodlanmış bağlantı özellikleri.
}

func encodeConnectionAttributes(cfg *Config) string {
	connAttrsBuf := make([]byte, 0)

	// varsayılan bağlantı özellikleri
	connAttrsBuf = appendLengthEncodedString(connAttrsBuf, connAttrClientName)
	connAttrsBuf = appendLengthEncodedString(connAttrsBuf, connAttrClientNameValue)
	connAttrsBuf = appendLengthEncodedString(connAttrsBuf, connAttrOS)
	connAttrsBuf = appendLengthEncodedString(connAttrsBuf, connAttrOSValue)
	connAttrsBuf = appendLengthEncodedString(connAttrsBuf, connAttrPlatform)
	connAttrsBuf = appendLengthEncodedString(connAttrsBuf, connAttrPlatformValue)
	connAttrsBuf = appendLengthEncodedString(connAttrsBuf, connAttrPid)
	connAttrsBuf = appendLengthEncodedString(connAttrsBuf, strconv.Itoa(os.Getpid()))
	serverHost, _, _ := net.SplitHostPort(cfg.Addr)
	if serverHost != "" {
		connAttrsBuf = appendLengthEncodedString(connAttrsBuf, connAttrServerHost)
		connAttrsBuf = appendLengthEncodedString(connAttrsBuf, serverHost)
	}

	// kullanıcı tanımlı bağlantı özellikleri
	for _, connAttr := range strings.Split(cfg.ConnectionAttributes, ",") {
		k, v, found := strings.Cut(connAttr, ":")
		if !found {
			continue
		}
		connAttrsBuf = appendLengthEncodedString(connAttrsBuf, k)
		connAttrsBuf = appendLengthEncodedString(connAttrsBuf, v)
	}

	return string(connAttrsBuf)
}

func newConnector(cfg *Config) *connector {
	encodedAttributes := encodeConnectionAttributes(cfg)
	return &connector{
		cfg:               cfg,
		encodedAttributes: encodedAttributes,
	}
}

// Connect, driver.Connector arayüzünü uygular.
// Connect, veritabanına bir bağlantı döndürür.
func (c *connector) Connect(ctx context.Context) (driver.Conn, error) {
	var err error

	// beforeConnect varsa çağır, yapılandırmanın bir kopyası ile
	cfg := c.cfg
	if c.cfg.beforeConnect != nil {
		cfg = c.cfg.Clone()
		err = c.cfg.beforeConnect(ctx, cfg)
		if err != nil {
			return nil, err
		}
	}

	// Yeni mysqlConn oluştur
	mc := &mysqlConn{
		maxAllowedPacket: maxPacketSize,
		maxWriteSize:     maxPacketSize - 1,
		closech:          make(chan struct{}),
		cfg:              cfg,
		connector:        c,
	}
	mc.parseTime = mc.cfg.ParseTime

	// Sunucuya Bağlan
	dctx := ctx
	if mc.cfg.Timeout > 0 {
		var cancel context.CancelFunc
		dctx, cancel = context.WithTimeout(ctx, c.cfg.Timeout)
		defer cancel()
	}

	if c.cfg.DialFunc != nil {
		mc.netConn, err = c.cfg.DialFunc(dctx, mc.cfg.Net, mc.cfg.Addr)
	} else {
		dialsLock.RLock()
		dial, ok := dials[mc.cfg.Net]
		dialsLock.RUnlock()
		if ok {
			mc.netConn, err = dial(dctx, mc.cfg.Addr)
		} else {
			nd := net.Dialer{}
			mc.netConn, err = nd.DialContext(dctx, mc.cfg.Net, mc.cfg.Addr)
		}
	}
	if err != nil {
		return nil, err
	}
	mc.rawConn = mc.netConn

	// TCP bağlantılarında TCP Keepalives etkinleştir
	if tc, ok := mc.netConn.(*net.TCPConn); ok {
		if err := tc.SetKeepAlive(true); err != nil {
			c.cfg.Logger.Print(err)
		}
	}

	// context desteği için startWatcher çağır (Go 1.8'den itibaren)
	mc.startWatcher()
	if err := mc.watchCancel(ctx); err != nil {
		mc.cleanup()
		return nil, err
	}
	defer mc.finish()

	mc.buf = newBuffer(mc.netConn)

	// G/Ç zaman aşımı ayarları
	mc.buf.timeout = mc.cfg.ReadTimeout
	mc.writeTimeout = mc.cfg.WriteTimeout

	// Handshake Initialization Packet okuma
	authData, plugin, err := mc.readHandshakePacket()
	if err != nil {
		mc.cleanup()
		return nil, err
	}

	if plugin == "" {
		plugin = defaultAuthPlugin
	}

	// İstemci Kimlik Doğrulama Paketi Gönder
	authResp, err := mc.auth(authData, plugin)
	if err != nil {
		// istenen eklentiyi kullanmak başarısız olursa varsayılan kimlik doğrulama eklentisini dene
		c.cfg.Logger.Print("istenen kimlik doğrulama eklentisi '"+plugin+"' kullanılamadı: ", err.Error())
		plugin = defaultAuthPlugin
		authResp, err = mc.auth(authData, plugin)
		if err != nil {
			mc.cleanup()
			return nil, err
		}
	}
	if err = mc.writeHandshakeResponsePacket(authResp, plugin); err != nil {
		mc.cleanup()
		return nil, err
	}

	// kimlik doğrulama paketine yanıtı işle, mümkünse yöntemleri değiştir
	if err = mc.handleAuthResult(authData, plugin); err != nil {
		// Kimlik doğrulama başarısız oldu ve MySQL bağlantıyı zaten kapattı
		// (https://dev.mysql.com/doc/internals/en/authentication-fails.html).
		// COM_QUIT göndermeyin, sadece temizleyin ve hatayı döndürün.
		mc.cleanup()
		return nil, err
	}

	if mc.cfg.MaxAllowedPacket > 0 {
		mc.maxAllowedPacket = mc.cfg.MaxAllowedPacket
	} else {
		// maksimum izin verilen paket boyutunu al
		maxap, err := mc.getSystemVar("max_allowed_packet")
		if err != nil {
			mc.Close()
			return nil, err
		}
		mc.maxAllowedPacket = stringToInt(maxap) - 1
	}
	if mc.maxAllowedPacket < maxPacketSize {
		mc.maxWriteSize = mc.maxAllowedPacket
	}

	// Charset: character_set_connection, character_set_client, character_set_results
	if len(mc.cfg.charsets) > 0 {
		for _, cs := range mc.cfg.charsets {
			// burada hataları yoksay - bir karakter seti mevcut olmayabilir
			if mc.cfg.Collation != "" {
				err = mc.exec("SET NAMES " + cs + " COLLATE " + mc.cfg.Collation)
			} else {
				err = mc.exec("SET NAMES " + cs)
			}
			if err == nil {
				break
			}
		}
		if err != nil {
			mc.Close()
			return nil, err
		}
	}

	// DSN Parametrelerini İşle
	err = mc.handleParams()
	if err != nil {
		mc.Close()
		return nil, err
	}

	return mc, nil
}

// Driver, driver.Connector arayüzünü uygular.
// Driver, &MySQLDriver{} döndürür.
func (c *connector) Driver() driver.Driver {
	return &MySQLDriver{}
}
