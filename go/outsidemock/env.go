package outsidemock

// Env は 偽の環境変数。渡した表だけを返す。
type Env struct {
	recorder
	vars map[string]string
}

// NewEnv は vars の写しを持つ Env を返す。
func NewEnv(vars map[string]string) *Env {
	m := make(map[string]string, len(vars))
	for k, v := range vars {
		m[k] = v
	}
	return &Env{vars: m}
}

func (e *Env) Lookup(name string) (string, bool) {
	e.record("Lookup", name)
	v, ok := e.vars[name]
	return v, ok
}
