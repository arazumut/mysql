// Go MySQL Sürücüsü - Go'nun database/sql paketi için bir MySQL Sürücüsü
//
// Telif Hakkı 2016 Go-MySQL-Driver Yazarlarına aittir. Tüm hakları saklıdır.
//
// Bu Kaynak Kod Formu, Mozilla Kamu Lisansı, v. 2.0 şartlarına tabidir. Bu dosya ile birlikte bir MPL kopyası dağıtılmadıysa,
// http://mozilla.org/MPL/2.0/ adresinden bir tane edinebilirsiniz.

package mysql

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"net"
	"testing"
)

func TestInterpolateParams(t *testing.T) {
	mc := &mysqlConn{
		buf:              newBuffer(nil),
		maxAllowedPacket: maxPacketSize,
		cfg: &Config{
			InterpolateParams: true,
		},
	}

	q, err := mc.interpolateParams("SELECT ?+?", []driver.Value{int64(42), "gopher"})
	if err != nil {
		t.Errorf("Beklenen err=nil, alınan %#v", err)
		return
	}
	expected := `SELECT 42+'gopher'`
	if q != expected {
		t.Errorf("Beklenen: %q\nAlınan: %q", expected, q)
	}
}

func TestInterpolateParamsJSONRawMessage(t *testing.T) {
	mc := &mysqlConn{
		buf:              newBuffer(nil),
		maxAllowedPacket: maxPacketSize,
		cfg: &Config{
			InterpolateParams: true,
		},
	}

	buf, err := json.Marshal(struct {
		Value int `json:"value"`
	}{Value: 42})
	if err != nil {
		t.Errorf("Beklenen err=nil, alınan %#v", err)
		return
	}
	q, err := mc.interpolateParams("SELECT ?", []driver.Value{json.RawMessage(buf)})
	if err != nil {
		t.Errorf("Beklenen err=nil, alınan %#v", err)
		return
	}
	expected := `SELECT '{\"value\":42}'`
	if q != expected {
		t.Errorf("Beklenen: %q\nAlınan: %q", expected, q)
	}
}

func TestInterpolateParamsTooManyPlaceholders(t *testing.T) {
	mc := &mysqlConn{
		buf:              newBuffer(nil),
		maxAllowedPacket: maxPacketSize,
		cfg: &Config{
			InterpolateParams: true,
		},
	}

	q, err := mc.interpolateParams("SELECT ?+?", []driver.Value{int64(42)})
	if err != driver.ErrSkip {
		t.Errorf("Beklenen err=driver.ErrSkip, alınan err=%#v, q=%#v", err, q)
	}
}

// Şu anda string literal içinde yer tutucu desteklemiyoruz.
// https://github.com/go-sql-driver/mysql/pull/490
func TestInterpolateParamsPlaceholderInString(t *testing.T) {
	mc := &mysqlConn{
		buf:              newBuffer(nil),
		maxAllowedPacket: maxPacketSize,
		cfg: &Config{
			InterpolateParams: true,
		},
	}

	q, err := mc.interpolateParams("SELECT 'abc?xyz',?", []driver.Value{int64(42)})
	// InterpolateParams string literal'ı desteklediğinde, bu `"SELECT 'abc?xyz', 42` döndürmelidir.
	if err != driver.ErrSkip {
		t.Errorf("Beklenen err=driver.ErrSkip, alınan err=%#v, q=%#v", err, q)
	}
}

func TestInterpolateParamsUint64(t *testing.T) {
	mc := &mysqlConn{
		buf:              newBuffer(nil),
		maxAllowedPacket: maxPacketSize,
		cfg: &Config{
			InterpolateParams: true,
		},
	}

	q, err := mc.interpolateParams("SELECT ?", []driver.Value{uint64(42)})
	if err != nil {
		t.Errorf("Beklenen err=nil, alınan err=%#v, q=%#v", err, q)
	}
	if q != "SELECT 42" {
		t.Errorf("Beklenen uint64 interpolasyonu çalışmalı, alınan q=%#v", q)
	}
}

func TestCheckNamedValue(t *testing.T) {
	value := driver.NamedValue{Value: ^uint64(0)}
	mc := &mysqlConn{}
	err := mc.CheckNamedValue(&value)

	if err != nil {
		t.Fatal("uint64 yüksek-bit dönüştürülemez", err)
	}

	if value.Value != ^uint64(0) {
		t.Fatalf("uint64 yüksek-bit dönüştürüldü, alınan %#v %T", value.Value, value.Value)
	}
}

// TestCleanCancel, başlangıçta iptal edilen bağlamı test eder.
// Hiçbir paket gönderilmemelidir. Bağlantı mevcut durumu korumalıdır.
func TestCleanCancel(t *testing.T) {
	mc := &mysqlConn{
		closech: make(chan struct{}),
	}
	mc.startWatcher()
	defer mc.cleanup()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	for i := 0; i < 3; i++ { // Aynı davranışı tekrarla
		err := mc.Ping(ctx)
		if err != context.Canceled {
			t.Errorf("beklenen context.Canceled, alınan %#v", err)
		}

		if mc.closed.Load() {
			t.Error("beklenen mc kapalı değil, aslında kapalı")
		}

		if mc.watching {
			t.Error("beklenen watching false, ama true")
		}
	}
}

func TestPingMarkBadConnection(t *testing.T) {
	nc := badConnection{err: errors.New("boom")}
	mc := &mysqlConn{
		netConn:          nc,
		buf:              newBuffer(nc),
		maxAllowedPacket: defaultMaxAllowedPacket,
		closech:          make(chan struct{}),
		cfg:              NewConfig(),
	}

	err := mc.Ping(context.Background())

	if err != driver.ErrBadConn {
		t.Errorf("beklenen driver.ErrBadConn, alınan  %#v", err)
	}
}

func TestPingErrInvalidConn(t *testing.T) {
	nc := badConnection{err: errors.New("failed to write"), n: 10}
	mc := &mysqlConn{
		netConn:          nc,
		buf:              newBuffer(nc),
		maxAllowedPacket: defaultMaxAllowedPacket,
		closech:          make(chan struct{}),
		cfg:              NewConfig(),
	}

	err := mc.Ping(context.Background())

	if err != nc.err {
		t.Errorf("beklenen %#v, alınan  %#v", nc.err, err)
	}
}

type badConnection struct {
	n   int
	err error
	net.Conn
}

func (bc badConnection) Write(b []byte) (n int, err error) {
	return bc.n, bc.err
}

func (bc badConnection) Close() error {
	return nil
}
