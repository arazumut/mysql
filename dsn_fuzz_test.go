//go:build go1.18
// +build go1.18

package mysql

import (
	"net"
	"testing"
)

func FuzzFormatDSN(f *testing.F) {
	// testDSNs listesindeki her bir test için f.Add fonksiyonunu çağır
	for _, test := range testDSNs { // dsn_test.go dosyasına bakın
		f.Add(test.in)
	}

	// Fuzz fonksiyonunu çağır

	f.Fuzz(func(t *testing.T, dsn1 string) {
		// Kaynakları boşa harcamayın
		if len(dsn1) > 1000 {
			t.Skip("yoksay: çok uzun")
		}

		// DSN'yi ayrıştır
		cfg1, err := ParseDSN(dsn1)
		if err != nil {
			t.Skipf("geçersiz DSN: %v", err)
		}

		// DSN'yi formatla
		dsn2 := cfg1.FormatDSN()
		if dsn2 == dsn1 {
			return
		}

		// ParseDSN tarafından sıkı bir şekilde kontrol edilmeyen kötü yapılandırma durumlarını yoksay
		if _, _, err := net.SplitHostPort(cfg1.Addr); err != nil {
			t.Skipf("geçersiz adres %q: %v", cfg1.Addr, err)
		}

		// DSN'yi tekrar ayrıştır
		cfg2, err := ParseDSN(dsn2)
		if err != nil {
			t.Fatalf("%q %q olarak yeniden yazıldı: %v", dsn1, dsn2, err)
		}

		// DSN'yi tekrar formatla
		dsn3 := cfg2.FormatDSN()
		if dsn3 != dsn2 {

			t.Errorf("%q %q olarak yeniden yazıldı", dsn2, dsn3)
		}
	})
}
