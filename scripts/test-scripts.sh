#!/usr/bin/env bash
# TC-OUT-SYNC・TC-OUT-RELEASE (docs/developer/spec.outside.yaml)。網を使わない: 取得元は test が作った bare repo。
# 作業は この repo の .work/ の下 (system の一時置き場を使わない)。
set -euo pipefail
here=$(cd "$(dirname "$0")" && pwd -P)
root=$(cd "$here/.." && pwd -P)
mkdir -p "$root/.work"
w=$(mktemp -d "$root/.work/scripts.XXXXXX")
trap 'rm -rf "$w"' EXIT
fail=0
ng() { echo "NG: $*"; fail=1; }
ok() { echo "ok: $*"; }
g() { git -c user.name=t -c user.email=t@example.invalid -c commit.gpgsign=false -c tag.gpgsign=false "$@"; }

# 取得元: 2 つの部品と それぞれの tag
src=$w/src
g init -q "$src"
mkdir -p "$src/lint-platform" "$src/go"
echo v1 > "$src/lint-platform/a.txt"; echo go1 > "$src/go/g.txt"
mkdir -p "$src/lint-platform/testdata" "$src/lint-platform/test" "$src/lint-platform/sub"
echo t > "$src/lint-platform/a_test.go"; echo t > "$src/lint-platform/testdata/x"; echo t > "$src/lint-platform/test/y_test.dart"
echo t > "$src/lint-platform/l.test.js"; echo t > "$src/lint-platform/sub/b_test.go"; echo keep > "$src/lint-platform/sub/b.go"
g -C "$src" add -A && g -C "$src" commit -qm 1
g -C "$src" tag lint-platform/v0.1.0
g -C "$src" tag go/v0.6.0
echo v2 > "$src/lint-platform/a.txt"
g -C "$src" commit -qam 2
g -C "$src" tag lint-platform/v0.2.0
g init -q --bare "$w/src.git"
g -C "$src" push -q "$w/src.git" --tags

# 取り込み先: 古い写しと余分な file
dst=$w/dst
g init -q "$dst"
mkdir -p "$dst/libs/outside/lint-platform"
echo old > "$dst/libs/outside/lint-platform/a.txt"; echo stray > "$dst/libs/outside/lint-platform/stray.txt"
snapshot() { (cd "$dst" && find libs -type f | sort | xargs shasum) ; }

# TC-OUT-SYNC: 正しい tag
(cd "$dst" && OUTSIDE_REPO="$w/src.git" "$here/sync-outside.sh" lint-platform lint-platform/v0.2.0 >/dev/null)
[ "$(cat "$dst/libs/outside/lint-platform/a.txt")" = v2 ] && ok "中身が tag と一致" || ng "中身が tag と違う"
[ ! -e "$dst/libs/outside/lint-platform/stray.txt" ] && ok "余分な file が消える" || ng "余分な file が残る"
[ "$(head -1 "$dst/libs/outside/lint-platform/.outside-version")" = lint-platform/v0.2.0 ] && ok ".outside-version の 1 行目 = tag" || ng ".outside-version の 1 行目が違う"
[ ! -e "$dst/libs/outside/go" ] && ok "他の部品を触らない" || ng "他の部品を置いた"
left=$(cd "$dst/libs/outside/lint-platform" && find . \( -name '*_test.go' -o -name '*.test.js' -o -name '*_test.dart' -o -name testdata -o -name test \) | sort | tr '\n' ' ')
[ -z "$left" ] && ok "部品の test を写さない" || ng "部品の test が写った: $left"
[ -f "$dst/libs/outside/lint-platform/sub/b.go" ] && ok "test でない file は写す" || ng "test でない file が消えた"
ls -d "$dst"/.outside-sync.* >/dev/null 2>&1 && ng "一時の dir が残る" || ok "一時の dir が残らない"

# TC-OUT-SYNC-CHECK: 網に行かない (取得元を退かせて回す)
mv "$w/src.git" "$w/src.git.away"
chk() { (cd "$dst" && OUTSIDE_REPO="$w/nowhere.git" "$here/sync-outside.sh" --check lint-platform 2>&1); }
lp=$dst/libs/outside/lint-platform
expect_rc() { # expect_rc <rc> <名指す語> <説明>
  local out rc=0; out=$(chk) || rc=$?
  if [ "$rc" = "$1" ] && { [ -z "$2" ] || printf '%s' "$out" | grep -qF -- "$2"; }; then ok "$3"; else ng "$3 (rc=$rc: $out)"; fi
}
grep -q '^fingerprint sha256:[0-9a-f]\{64\}$' "$lp/.outside-version" && ok "2 行目 = 全体の指紋" || ng "指紋の行が無い"
expect_rc 0 "指紋が一致" "取り込んだ直後は一致"
cp "$lp/a.txt" "$w/a.keep"; echo edited > "$lp/a.txt"
expect_rc 1 "違う libs/outside/lint-platform/a.txt" "中身を変えた file を名指す"
cp "$w/a.keep" "$lp/a.txt"; echo x > "$lp/added.txt"
expect_rc 1 "増えた libs/outside/lint-platform/added.txt" "足した file を名指す"
rm "$lp/added.txt"; mv "$lp/sub/b.go" "$w/b.keep"
expect_rc 1 "消えた libs/outside/lint-platform/sub/b.go" "消した file を名指す"
mv "$w/b.keep" "$lp/sub/b.go"
expect_rc 0 "" "戻せば一致"
cp "$lp/.outside-version" "$w/ver.keep"
sed -i.bak 's/^file [0-9a-f]* a.txt$/file 0000000000000000000000000000000000000000000000000000000000000000 a.txt/' "$lp/.outside-version"; rm -f "$lp/.outside-version.bak"
expect_rc 1 "記録が書き換わっている" "file ごとの行の書き換えを捕る"
head -1 "$w/ver.keep" > "$lp/.outside-version"
expect_rc 3 "指紋の行が無い" "指紋の無い写しは 判定不能"
cp "$w/ver.keep" "$lp/.outside-version"
if (cd "$dst" && "$here/sync-outside.sh" --check go >/dev/null 2>&1); then ng "置き場が無いのに通った"; else
  rc=0; (cd "$dst" && "$here/sync-outside.sh" --check go >/dev/null 2>&1) || rc=$?
  [ "$rc" = 3 ] && ok "置き場が無い は 判定不能" || ng "置き場が無い の rc=$rc"; fi
mv "$w/src.git.away" "$w/src.git"

# TC-OUT-SYNC: 誤りの時は 1 byte も変えない
before=$(snapshot)
for args in "lint-platform go/v0.6.0" "lint-platform v0.6.0" "lint-platform lint-platform/v9.9.9" "nope nope/v1.0.0"; do
  if (cd "$dst" && OUTSIDE_REPO="$w/src.git" "$here/sync-outside.sh" $args >/dev/null 2>&1); then ng "誤りが通った: $args"; fi
  [ "$(snapshot)" = "$before" ] && ok "誤り ($args) で 置き場が変わらない" || ng "誤り ($args) で 置き場が変わった"
done

# TC-OUT-RELEASE
rel() { (cd "$src" && "$here/release.sh" "$1" "$2" --print); }
[ "$(rel lint-platform minor)" = lint-platform/v0.3.0 ] && ok "lint-platform minor" || ng "lint-platform minor = $(rel lint-platform minor)"
[ "$(rel go patch)" = go/v0.6.1 ] && ok "go patch (他の部品の tag に影響されない)" || ng "go patch = $(rel go patch)"
[ "$(rel lint-dart major)" = lint-dart/v0.1.0 ] && ok "tag の無い部品 = v0.1.0" || ng "lint-dart = $(rel lint-dart major)"
g -C "$src" tag lint-platform/v0.10.0
[ "$(rel lint-platform patch)" = lint-platform/v0.10.1 ] && ok "版は数で比べる (0.10 > 0.2)" || ng "版の比べ方 = $(rel lint-platform patch)"

g -C "$src" tag v9.9.9
if rel lint-platform patch >/dev/null 2>&1; then ng "部品の形でない tag が在るのに 版を出した"; else ok "部品の形でない tag が在れば止まる"; fi
g -C "$src" tag -d v9.9.9 >/dev/null

exit $fail
