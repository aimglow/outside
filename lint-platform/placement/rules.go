package placement

import (
	"regexp"

	"github.com/aimglow/outside/lint-platform/naming"
)

// 本物の口を作る物の表 (規則の正本。説明 = docs/developer/design.outside.yaml の lint.platform.tests.real_use)。
// outside の口が増えたら 門番が ここに足す (go/real.go・next と angular の src/index.ts・flutter/lib/outside.dart の 公開の本物)。

// OutsideGoImport は outside の Go の部品の import path、OutsideGoPackage は その package の名前。
const (
	OutsideGoImport  = "github.com/aimglow/outside/go"
	OutsideGoPackage = "outside"
)

// OutsideGoReal は outside の Go の部品が 本物を作る 公開の名前 (go/real.go)。
var OutsideGoReal = []string{
	"RunTool", "NewClock", "NewEnv", "NewFS", "NewProc", "NewWorkdir", "NewSysInfo", "NewIdGen", "NewRandom",
}

// OutsideTSPackages は outside の TS の部品の specifier (その下の path も含む)。
var OutsideTSPackages = []string{"@aimglow/outside-next", "@aimglow/outside-angular"}

// OutsideTSPlaces は outside の TS の部品の 置き場 (相対 path の import が ここを指せば outside の import)。
var OutsideTSPlaces = []string{"libs/outside/next", "libs/outside/angular"}

// OutsideTSRealPrefix は outside の TS の部品が 本物を作る関数の名前の頭。
const OutsideTSRealPrefix = "createReal"

// OutsideDartImportPrefix は outside の Dart の部品の import の URI の頭。
const OutsideDartImportPrefix = "package:outside/"

// outsideDartReal は outside の Dart の部品で 本物を作る形 (RealXxx( の呼び出し・OutsideSet.real)。
var outsideDartReal = regexp.MustCompile(`\b(Real[A-Z]\w*)\s*\(|\b(OutsideSet\s*\.\s*real)\b`)

// AdapterRealFiles は runtime (go・ts・dart) ごとの adapters の本物の file の名前 (lint.platform の rules の雛形の決まり)。
var AdapterRealFiles = map[string]string{
	"go":   naming.RequiredFiles["go"][1],
	"ts":   naming.RequiredFiles["next"][1],
	"dart": naming.RequiredFiles["flutter"][1],
}
