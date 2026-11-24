#!/usr/bin/env bash
# sync-outside.sh <部品> <tag>      — outside の部品を 版の tag から取り出し この project の置き場を丸ごと置き換える。
# sync-outside.sh --check <部品>    — 置き場の中身が .outside-version の指紋どおりかを 網に行かずに確かめる。
# project の repo の根で実行する。部品と置き場の対応 = outside の docs/developer/design.outside.yaml の components。
# 取得元 = https://github.com/aimglow/outside.git (OUTSIDE_REPO で差し替えられる。test は test が作った bare repo を渡す)。
# GitHub は git archive --remote に応じないので 一時の bare repo に その tag だけを取り そこから git archive する。
# 部品の test (Go の *_test.go・testdata/、TS の *.test.*・*.spec.ts、Dart の *_test.dart・test/) は写さない。
# .outside-version = 1 行目 tag / 2 行目 fingerprint sha256:<全体> / 続く行 file <sha256> <path> (置き場の file ごと、path の順)。
# 取り込みの誤りの時は 置き場を 1 byte も変えずに exit 1 で終わる。
# --check の exit = 0 一致 / 1 違う (違う file を名指す) / 3 判定不能 (置き場・.outside-version・指紋の行が無い)。
set -euo pipefail

die() { echo "sync-outside: $*" >&2; exit 1; }
undetermined() { echo "sync-outside: 判定不能: $*" >&2; exit 3; }
usage="使い方 = sync-outside.sh <部品> <tag> (例 sync-outside.sh lint-platform lint-platform/v0.1.0) / sync-outside.sh --check <部品>"
[ $# -eq 2 ] || die "$usage"
mode=sync
if [ "$1" = --check ]; then mode=check; shift; set -- "$1" ""; fi
comp=$1 tag=$2
case $comp in
  go|next|angular|flutter|lint-go|lint-ts|lint-dart|lint-platform) src=$comp dest=libs/outside/$comp ;;
  docs) src=docs/deterministic-testing dest=docs/deterministic-testing ;;
  *) die "知らない部品 $comp (go|next|angular|flutter|lint-go|lint-ts|lint-dart|lint-platform|docs)" ;;
esac
git rev-parse --show-toplevel >/dev/null 2>&1 || die "git の repo の中で実行する"
[ "$(git rev-parse --show-toplevel)" = "$(pwd -P)" ] || die "repo の根で実行する"

sha() { if command -v sha256sum >/dev/null 2>&1; then sha256sum; else shasum -a 256; fi | cut -d' ' -f1; }

# manifest <dir> は dir の中の file (.outside-version を除く。symlink は 指す先の文字列) を path の順に 『file <sha256> <path>』 で出す。
manifest() {
  (cd "$1" && find . ! -type d ! -path ./.outside-version -print | sed 's#^\./##' | LC_ALL=C sort |
    while IFS= read -r f; do
      if [ -L "$f" ]; then h=$(printf '%s' "$(readlink "$f")" | sha); else h=$(sha <"$f"); fi
      printf 'file %s %s\n' "$h" "$f"
    done)
}

if [ $mode = check ]; then
  [ -d "$dest" ] || undetermined "$dest が無い (取り込んでいない)"
  ver=$dest/.outside-version
  [ -f "$ver" ] || undetermined "$ver が無い"
  rec_tag=$(sed -n 1p "$ver")
  rec_fp=$(sed -n 2p "$ver")
  [[ $rec_fp =~ ^fingerprint\ sha256:[0-9a-f]{64}$ ]] || undetermined "$ver に指紋の行が無い (指紋を書く前の sync-outside.sh で置いた写し。sync-outside.sh $comp ${rec_tag:-<tag>} で取り込み直す)"
  [[ $rec_tag =~ ^${comp}/v[0-9]+\.[0-9]+\.[0-9]+$ ]] || { echo "sync-outside: $ver の 1 行目 $rec_tag は 部品 $comp の tag でない" >&2; exit 1; }
  recorded=$(sed -n '3,$p' "$ver")
  fail=0
  if [ "fingerprint sha256:$(printf '%s\n' "$recorded" | sha)" != "$rec_fp" ]; then
    echo "sync-outside: 違う $ver (file ごとの行が 全体の指紋と合わない = 記録が書き換わっている)" >&2; fail=1
  fi
  actual=$(manifest "$dest")
  # path ごとに 記録と今を突き合わせる (path は 3 つ目の空白の後ろ全部)。
  diffs=$(LC_ALL=C awk '
    FNR == 1 { side++ }
    $0 == "" { next }
    { p = $0; sub(/^file [0-9a-f]+ /, "", p); h = $2
      if (side == 1) rec[p] = h; else act[p] = h }
    END {
      for (p in rec) { if (!(p in act)) print "消えた " p; else if (rec[p] != act[p]) print "違う " p }
      for (p in act) if (!(p in rec)) print "増えた " p
    }' <(printf '%s\n' "$recorded") <(printf '%s\n' "$actual") | LC_ALL=C sort -k2)
  if [ -n "$diffs" ]; then
    printf '%s\n' "$diffs" | sed "s#^\([^ ]*\) #sync-outside: \1 $dest/#" >&2; fail=1
  fi
  [ $fail = 0 ] || { echo "sync-outside: $dest は $rec_tag の写しと違う (写しは編集しない。直すなら sync-outside.sh $comp $rec_tag)" >&2; exit 1; }
  echo "sync-outside: $dest = $rec_tag (指紋が一致)"
  exit 0
fi

[[ $tag =~ ^${comp}/v[0-9]+\.[0-9]+\.[0-9]+$ ]] || die "tag $tag は 部品 $comp の版の形 ($comp/vX.Y.Z) でない"

repo=${OUTSIDE_REPO:-https://github.com/aimglow/outside.git}
work=$(mktemp -d "$PWD/.outside-sync.XXXXXX")
trap 'rm -rf "$work"' EXIT
git init -q --bare "$work/repo.git"
git -C "$work/repo.git" fetch -q --depth 1 "$repo" "refs/tags/$tag:refs/tags/$tag" 2>/dev/null || die "$repo に tag $tag が無い (取れない)"
mkdir "$work/out"
git -C "$work/repo.git" archive "$tag" "$src" 2>/dev/null | tar -x -C "$work/out" || die "tag $tag に $src が無い"
[ -d "$work/out/$src" ] || die "tag $tag に $src が無い"
# 部品の test は project で回さないので 写さない (project の test の検査に 部品の test を混ぜない)。
find "$work/out/$src" \( -name '*_test.go' -o -name '*.test.js' -o -name '*.test.ts' -o -name '*.spec.ts' -o -name '*_test.dart' \) -type f -delete
find "$work/out/$src" -depth -type d \( -name testdata -o -name test \) -exec rm -rf {} +
files=$(manifest "$work/out/$src")
{ printf '%s\n' "$tag"; printf 'fingerprint sha256:%s\n' "$(printf '%s\n' "$files" | sha)"; printf '%s\n' "$files"; } > "$work/out/$src/.outside-version"

mkdir -p "$(dirname "$dest")"
if [ -e "$dest" ]; then mv "$dest" "$work/old"; fi
mv "$work/out/$src" "$dest"
echo "sync-outside: $dest = $tag"
