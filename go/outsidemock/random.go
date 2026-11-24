package outsidemock

import (
	"crypto/sha256"
	"encoding/binary"
	"strconv"
	"sync"
)

// Random は 種 (seed) で決まる byte 列を返す mock。同じ種・同じ呼び順なら 同じ byte。並行に呼んでも安全。
// 列は sha256(種 || 番号 (big endian 8 byte)) の block を 番号 0 から並べた物。分けて読んでも 続けて読んでも 同じ列。
type Random struct {
	recorder
	mu    sync.Mutex
	seed  []byte
	block uint64
	buf   []byte
	err   error
}

// NewRandom は seed で決まる列を返す Random を作る。
func NewRandom(seed []byte) *Random {
	return &Random{seed: append([]byte(nil), seed...)}
}

// WithErr は 以後の Read が n=0 と err を返すようにする (呼び手の誤りの経路を test する)。nil なら 列を返す形に戻す。
func (r *Random) WithErr(err error) *Random {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.err = err
	return r
}

// Read は 記録して 列の続きで p を埋める。記録の Args は 読んだ長さ。
func (r *Random) Read(p []byte) (int, error) {
	r.record("Read", strconv.Itoa(len(p)))
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.err != nil {
		return 0, r.err
	}
	n := 0
	for n < len(p) {
		if len(r.buf) == 0 {
			var ctr [8]byte
			binary.BigEndian.PutUint64(ctr[:], r.block)
			sum := sha256.Sum256(append(append([]byte(nil), r.seed...), ctr[:]...))
			r.buf = sum[:]
			r.block++
		}
		c := copy(p[n:], r.buf)
		r.buf = r.buf[c:]
		n += c
	}
	return n, nil
}
