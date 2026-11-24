package naming

import (
	"fmt"
	"strings"
)

// Scaffold は 規則どおりの lib の名前と 3 file の雛形 (相対 path → 中身) を返す。名前が規則に合わなければ 誤り。disk を触らない。
func Scaffold(v Vocab, capability, dep, runtime string) (string, map[string]string, error) {
	name := capability + "-" + dep + "-" + runtime
	if _, _, _, reason := v.ParseName(name); reason != "" {
		return "", nil, fmt.Errorf("%s: %s", name, reason)
	}
	files := map[string]string{}
	f := RequiredFiles[runtime]
	switch runtime {
	case "go":
		pkg := strings.ReplaceAll(capability+dep, "-", "")
		files[f[0]] = "// Package " + pkg + " は " + dep + " を触る口 (interface)。外を直に呼ぶのは " + f[1] + " だけ。\npackage " + pkg + "\n\n// Port は " + dep + " へ頼む事の口。\ntype Port interface {\n}\n"
		files[f[1]] = "package " + pkg + "\n\n// Real は " + dep + " の本物を呼ぶ実装。\ntype Real struct{}\n\nvar _ Port = (*Real)(nil)\n"
		files[f[2]] = "package " + pkg + "\n\n// Mock は 渡した値だけで振る舞いが決まる実装。\ntype Mock struct{}\n\nvar _ Port = (*Mock)(nil)\n"
	case "next", "angular":
		files["src/"+f[0]] = "// " + dep + " へ頼む事の口。外を直に呼ぶのは real.ts だけ。\nexport interface Port {}\n"
		files["src/"+f[1]] = "import type { Port } from './interface';\n\n// " + dep + " の本物を呼ぶ実装。\nexport class Real implements Port {}\n"
		files["src/"+f[2]] = "import type { Port } from './interface';\n\n// 渡した値だけで振る舞いが決まる実装。\nexport class Mock implements Port {}\n"
	case "flutter":
		files["lib/"+f[0]] = "/// " + dep + " へ頼む事の口。外を直に呼ぶのは adapter_real.dart だけ。\nabstract interface class Port {}\n"
		files["lib/"+f[1]] = "import 'interface.dart';\n\n/// " + dep + " の本物を呼ぶ実装。\nclass Real implements Port {}\n"
		files["lib/"+f[2]] = "import 'interface.dart';\n\n/// 渡した値だけで振る舞いが決まる実装。\nclass Mock implements Port {}\n"
	}
	return name, files, nil
}
