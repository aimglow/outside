// Package outsidemock は outside の口のmock (mock) を持つ。口 1 つにつき 1 つ。
// mockは test が渡した値だけで振る舞いが決まり 呼ばれた内容を記録する。外は触らない。
package outsidemock

import "sync"

// Call は mockが受けた呼び出し 1 件。
type Call struct {
	Method string
	Args   []string
}

// CallsOf は calls から method の呼び出しだけを抜く。
func CallsOf(calls []Call, method string) []Call {
	var out []Call
	for _, c := range calls {
		if c.Method == method {
			out = append(out, c)
		}
	}
	return out
}

// recorder は 呼び出しの記録。各mockが埋め込む。
type recorder struct {
	recMu sync.Mutex
	calls []Call
}

func (r *recorder) record(method string, args ...string) {
	r.recMu.Lock()
	defer r.recMu.Unlock()
	r.calls = append(r.calls, Call{Method: method, Args: args})
}

// Calls は 受けた呼び出しを 順に返す。
func (r *recorder) Calls() []Call {
	r.recMu.Lock()
	defer r.recMu.Unlock()
	return append([]Call(nil), r.calls...)
}
