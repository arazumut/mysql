// Go MySQL Driver - Go'nun database/sql paketi için bir MySQL Sürücüsü
//
// Telif Hakkı 2013 Go-MySQL-Driver Yazarlarına aittir. Tüm hakları saklıdır.
//
// Bu Kaynak Kod Formu, Mozilla Kamu Lisansı, v. 2.0 şartlarına tabidir.
// Bu dosya ile birlikte MPL'nin bir kopyası dağıtılmadıysa,
// http://mozilla.org/MPL/2.0/ adresinden bir kopyasını edinebilirsiniz.

package mysql

import (
	"io"
	"net"
	"time"
)

const defaultBufSize = 4096
const maxCachedBufSize = 256 * 1024

// Hem okuma hem de yazma için kullanılan bir tampon.
// Bu, her bağlantıdaki iletişimin senkron olması nedeniyle mümkündür.
// Başka bir deyişle, aynı bağlantıda aynı anda yazma ve okuma yapamayız.
// Tampon, bufio.Reader / Writer'a benzer ancak sıfır kopya benzeri
// Ayrıca bu özel kullanım durumu için yüksek derecede optimize edilmiştir.
type buffer struct {
	buf       []byte // okuma tamponu.
	cachedBuf []byte // yeniden kullanılacak tampon. len(cachedBuf) <= maxCachedBufSize.
	nc        net.Conn
	timeout   time.Duration
}

// newBuffer yeni bir tampon ayırır ve döndürür.
func newBuffer(nc net.Conn) buffer {
	return buffer{
		cachedBuf: make([]byte, defaultBufSize),
		nc:        nc,
	}
}

// busy okuma tamponu boş değilse true döner.
func (b *buffer) busy() bool {
	return len(b.buf) > 0
}

// fill okuma tamponunu en az _need_ bayt olana kadar doldurur.
func (b *buffer) fill(need int) error {
	// mevcut tamponun içeriğini doldurmadan önce dest'e taşıyacağız.
	dest := b.cachedBuf

	// tüm paketi sığdırmak için gerekirse tamponu büyütün.
	if need > len(dest) {
		// Varsayılan boyutun bir sonraki katına yuvarlayın
		dest = make([]byte, ((need/defaultBufSize)+1)*defaultBufSize)

		// ayrılan tampon çok büyük değilse, ekstra tahsisleri önlemek için
		// arka depolamaya taşıyın
		if len(dest) <= maxCachedBufSize {
			b.cachedBuf = dest
		}
	}

	// mevcut verileri tamponun başlangıcına taşıyın.
	n := len(b.buf)
	copy(dest[:n], b.buf)

	for {
		if b.timeout > 0 {
			if err := b.nc.SetReadDeadline(time.Now().Add(b.timeout)); err != nil {
				return err
			}
		}

		nn, err := b.nc.Read(dest[n:])
		n += nn

		if err == nil && n < need {
			continue
		}

		b.buf = dest[:n]

		if err == io.EOF {
			if n < need {
				err = io.ErrUnexpectedEOF
			} else {
				err = nil
			}
		}
		return err
	}
}

// tampondan sonraki N baytı döndürür.
// Döndürülen dilim, yalnızca bir sonraki okumaya kadar geçerli olacağı garanti edilir
func (b *buffer) readNext(need int) ([]byte, error) {
	if len(b.buf) < need {
		// yeniden doldur
		if err := b.fill(need); err != nil {
			return nil, err
		}
	}

	data := b.buf[:need]
	b.buf = b.buf[need:]
	return data, nil
}

// istenen boyutta bir tampon döndürür.
// Mümkünse, mevcut tampondan bir dilim döndürülür.
// Aksi takdirde daha büyük bir tampon yapılır.
// Aynı anda yalnızca bir tampon (toplam) kullanılabilir.
func (b *buffer) takeBuffer(length int) ([]byte, error) {
	if b.busy() {
		return nil, ErrBusyBuffer
	}

	// önce (ucuz) genel durumu test edin
	if length <= len(b.cachedBuf) {
		return b.cachedBuf[:length], nil
	}

	if length < maxCachedBufSize {
		b.cachedBuf = make([]byte, length)
		return b.cachedBuf, nil
	}

	// tampon saklamak istediğimizden daha büyük.
	return make([]byte, length), nil
}

// takeSmallBuffer, uzunluğun
// varsayılanBufSize'dan küçük olduğu biliniyorsa kullanılabilecek bir kısayoldur.
// Aynı anda yalnızca bir tampon (toplam) kullanılabilir.
func (b *buffer) takeSmallBuffer(length int) ([]byte, error) {
	if b.busy() {
		return nil, ErrBusyBuffer
	}
	return b.cachedBuf[:length], nil
}

// takeCompleteBuffer mevcut tamponun tamamını döndürür.
// Gerekli tampon boyutu bilinmiyorsa kullanılabilir.
// Döndürülen tamponun kap ve uzunluğu eşit olacaktır.
// Aynı anda yalnızca bir tampon (toplam) kullanılabilir.
func (b *buffer) takeCompleteBuffer() ([]byte, error) {
	if b.busy() {
		return nil, ErrBusyBuffer
	}
	return b.cachedBuf, nil
}

// store, buf'u, güncellenmiş bir tamponu, uygun olduğu takdirde depolar.
func (b *buffer) store(buf []byte) {
	if cap(buf) <= maxCachedBufSize && cap(buf) > cap(b.cachedBuf) {
		b.cachedBuf = buf[:cap(buf)]
	}
}
