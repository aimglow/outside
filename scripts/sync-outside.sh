#!/usr/bin/env bash
# sync-outside.sh <部品> <tag> — outside の部品を 版の tag から取り出し この project の置き場を丸ごと置き換える。
# project の repo の根で実行する。部品と置き場の対応 = outside の docs/developer/design.outside.yaml の components。
# 取得元 = https://github.com/aimglow/outside.git (OUTSIDE_REPO で差し替えられる。test は test が作った bare repo を渡す)。
# GitHub は git archive --remote に応じないので 一時の bare repo に その tag だけを取り そこから git archive する。
# 誤りの時は 置き場を 1 byte も変えずに exit 1 で終わる。
set -euo pipefail

die() { echo "sync-outside: $*" >&2; exit 1; }
[ $# -eq 2 ] || die "使い方 = sync-outside.sh <部品> <tag> (例 sync-outside.sh lint-platform lint-platform/v0.1.0)"
comp=$1 tag=$2
case $comp in
  go|next|angular|flutter|lint-go|lint-ts|lint-dart|lint-platform) src=$comp dest=libs/outside/$comp ;;
  docs) src=docs/deterministic-testing dest=docs/deterministic-testing ;;
  *) die "知らない部品 $comp (go|next|angular|flutter|lint-go|lint-ts|lint-dart|lint-platform|docs)" ;;
esac
[[ $tag =~ ^${comp}/v[0-9]+\.[0-9]+\.[0-9]+$ ]] || die "tag $tag は 部品 $comp の版の形 ($comp/vX.Y.Z) でない"
git rev-parse --show-toplevel >/dev/null 2>&1 || die "git の repo の中で実行する"
[ "$(git rev-parse --show-toplevel)" = "$(pwd -P)" ] || die "repo の根で実行する"

repo=${OUTSIDE_REPO:-https://github.com/aimglow/outside.git}
work=$(mktemp -d "$PWD/.outside-sync.XXXXXX")
trap 'rm -rf "$work"' EXIT
git init -q --bare "$work/repo.git"
git -C "$work/repo.git" fetch -q --depth 1 "$repo" "refs/tags/$tag:refs/tags/$tag" 2>/dev/null || die "$repo に tag $tag が無い (取れない)"
mkdir "$work/out"
git -C "$work/repo.git" archive "$tag" "$src" 2>/dev/null | tar -x -C "$work/out" || die "tag $tag に $src が無い"
[ -d "$work/out/$src" ] || die "tag $tag に $src が無い"
printf '%s\n' "$tag" > "$work/out/$src/.outside-version"

mkdir -p "$(dirname "$dest")"
if [ -e "$dest" ]; then mv "$dest" "$work/old"; fi
mv "$work/out/$src" "$dest"
echo "sync-outside: $dest = $tag"
