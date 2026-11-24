#!/usr/bin/env bash
# release.sh <部品> <major|minor|patch> [--print]
# その部品の最新の tag (<部品>/vX.Y.Z) から次の版を決め 部品の test を回し tag を打って push する。
# 他の部品の tag には影響されない。tag の無い部品は <部品>/v0.1.0。--print は 次の tag を出すだけ (test も tag もしない)。
set -euo pipefail

die() { echo "release: $*" >&2; exit 1; }
[ $# -ge 2 ] || die "使い方 = release.sh <部品> <major|minor|patch> [--print]"
comp=$1 level=$2 print=${3:-}
case $comp in go|next|angular|flutter|lint-go|lint-ts|lint-dart|lint-platform) dir=$comp ;; docs) dir=docs/deterministic-testing ;;
  *) die "知らない部品 $comp" ;; esac
case $level in major|minor|patch) ;; *) die "段は major|minor|patch" ;; esac
cd "$(git rev-parse --show-toplevel)"

# 出ている版の正本は origin の tag。打つ前に取り直し origin に無い tag を手元から消す。
if git remote get-url origin >/dev/null 2>&1; then git fetch -q --tags --prune-tags origin; fi
# 部品の形でない tag は作らない決まり。1 つでも在れば 何もせず止まる (紛れた tag の上に版を積まない)。
bad=$(git tag -l | grep -vE '^(go|next|angular|flutter|lint-go|lint-ts|lint-dart|lint-platform|docs)/v[0-9]+\.[0-9]+\.[0-9]+$' || true)
[ -z "$bad" ] || die "部品の形 (<部品>/vX.Y.Z) でない tag が在る: $(echo $bad)。消してから やり直す"

last=$(git tag -l "$comp/v*" | sed "s#^$comp/v##" | grep -E '^[0-9]+\.[0-9]+\.[0-9]+$' | sort -t. -k1,1n -k2,2n -k3,3n | tail -1 || true)
if [ -z "$last" ]; then next=0.1.0; else
  IFS=. read -r ma mi pa <<<"$last"
  case $level in major) next=$((ma+1)).0.0 ;; minor) next=$ma.$((mi+1)).0 ;; patch) next=$ma.$mi.$((pa+1)) ;; esac
fi
tag=$comp/v$next
if [ "$print" = "--print" ]; then echo "$tag"; exit 0; fi

[ -d "$dir" ] || die "部品の dir $dir が無い"
[ -z "$(git status --porcelain)" ] || die "作業木に commit していない変更がある"
case $comp in
  go|lint-go|lint-platform) (cd "$dir" && go vet ./... && go test -count=1 ./...) ;;
  lint-ts|next|angular) (cd "$dir" && pnpm install --frozen-lockfile && pnpm test) ;;
  lint-dart|flutter) (cd "$dir" && dart pub get && dart test) ;;
  docs) ;;
esac
git tag "$tag"
git push origin "$tag"
echo "RELEASE_SUCCESS: $tag"
