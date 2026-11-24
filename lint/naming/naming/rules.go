// Package naming は adapters 層 (libs/platform) の 名前と構成の規則を持つ (規則の正本)。
// 正本の説明 = docs/deterministic-testing/reference.deterministic-testing.adapters.md §0。
package naming

import "regexp"

// Capabilities は <機能> に使ってよい語。語を足すときは ここを直して 版を上げる。
var Capabilities = []string{
	"authn", "db", "vcs", "container", "migrate", "schema",
	"config", "log", "http", "kv", "ipc", "remote",
}

// Runtimes は <runtime> に使ってよい語 (outside の target と同じ)。
var Runtimes = []string{"go", "next", "angular", "flutter"}

// RequiredFiles は runtime ごとに lib が持たなければならない 3 file (interface / real / mock)。
var RequiredFiles = map[string][]string{
	"go":      {"interface.go", "adapter_real.go", "adapter_mock.go"},
	"next":    {"interface.ts", "real.ts", "mock.ts"},
	"angular": {"interface.ts", "real.ts", "mock.ts"},
	"flutter": {"interface.dart", "adapter_real.dart", "adapter_mock.dart"},
}

// AllowedSubdirs は lib の直下に置いてよい dir。3 file は lib の直下か ここに置く。
var AllowedSubdirs = []string{"src", "lib", "test", "testdata"}

// namePattern は lib の dir 名の形 = <機能>-<依存先>-<runtime>。
var namePattern = regexp.MustCompile(
	`^(authn|db|vcs|container|migrate|schema|config|log|http|kv|ipc|remote)-[a-z0-9]+(-[a-z0-9]+)*-(go|next|angular|flutter)$`)
