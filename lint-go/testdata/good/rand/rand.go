// Package rand は 名前だけが標準の乱数と同じ 自作の package (lint は import path で区切り 落とさない)。
package rand

// Intn は 決まった値を返す (乱数ではない)。
func Intn(n int) int { return n - 1 }
