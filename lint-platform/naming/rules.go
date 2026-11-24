// Package naming は adapters 層 (各 project の libs/platform) の 名前と構成の規則を持つ (規則の正本)。
// 説明 = docs/developer/design.outside.yaml の lint.platform。
package naming

// Runtimes は <runtime> に使ってよい語 (outside の口の部品と 1 対 1)。足すのは outside 門番。
var Runtimes = []string{"go", "next", "angular", "flutter"}

// CommonCapabilities は 全 project 共通の <機能> の語。足すのは outside 門番 (2 つ以上の project が使う語)。
// project 固有の語は その project の repo の根の outside-platform.toml の capabilities に書く。
var CommonCapabilities = []string{
	"authn", "db", "vcs", "container", "migrate", "schema",
	"config", "log", "http", "kv", "ipc", "remote",
}

// VagueWords は 中身を表さない語。共通の表にも project の宣言にも 置けない。
var VagueWords = []string{"common", "util", "utils", "shared", "helper", "helpers", "misc", "lib", "core", "base", "other"}

// RequiredFiles は runtime ごとに lib が持つ 3 file (interface / real / mock)。
var RequiredFiles = map[string][]string{
	"go":      {"interface.go", "adapter_real.go", "adapter_mock.go"},
	"next":    {"interface.ts", "real.ts", "mock.ts"},
	"angular": {"interface.ts", "real.ts", "mock.ts"},
	"flutter": {"interface.dart", "adapter_real.dart", "adapter_mock.dart"},
}

// FileDirs は 3 file を置いてよい場所 (lib の直下・src・lib)。
var FileDirs = []string{"", "src", "lib"}
