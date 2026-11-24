# 09. 運用フロー・マイクロ CI/CD および同期スクリプト集

本章では、`outside` 専用セッションへの `relay` 運用、ローカル完結型のマイクロ CI/CD スクリプト（`release.sh`）、および Nx ワークスペース内の `libs/share/outside/{go,angular,next,flutter}` へクリーンに展開する同期スクリプト（`sync-outside.sh`）を定義します。

---

## 1. ディレクトリ構成の前提

```text
# 隣接配置された outside リポジトリ
../outside/
├── AGENTS.md                  # 08_agents_md_outside_gatekeeper.md を配置
├── scripts/
│   └── release.sh             # 全言語テスト＆自動タグ発行スクリプト
├── go/                        # Go 実装・テスト
├── next/                      # Next.js (Node.js) 実装・テスト
├── angular/                   # Angular (Browser) 実装・テスト
└── flutter/                   # Flutter (Pure Dart) 実装・テスト

# 本体 Nx ワークスペース
./workspace/
├── AGENTS.md                  # 07_agents_md_app_template.md を配置
├── scripts/
│   └── sync-outside.sh        # git archive によるクリーン同期スクリプト
└── libs/share/outside/
    ├── go/
    ├── next/
    ├── angular/
    └── flutter/
```

---

## 2. `outside` リポジトリ用：`scripts/release.sh`（マイクロ CI/CD）

全 4 区分（`go`, `next`, `angular`, `flutter`）のテストを実行し、並行作業時のタグ衝突を自動リトライで回避しながらパッチバージョンをインクリメントして Push します。

```bash
#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_ROOT"

# 1. 未コミット変更の検知
if [[ -n "$(git status --porcelain)" ]]; then
  echo "ERROR: 未コミットの変更があります。先に git commit してください。" >&2
  git status --short >&2
  exit 1
fi

# 2. 全ランタイムのテスト実行（Real & Fake 両面検証）
echo "==> Running tests across all runtime targets..."

if [[ -d "go" ]]; then
  echo "--> [go] go test -count=1 ./..."
  (cd go && go test -count=1 ./...)
fi

if [[ -d "next" ]]; then
  echo "--> [next] npm test"
  (cd next && npm test)
fi

if [[ -d "angular" ]]; then
  echo "--> [angular] npm test"
  (cd angular && npm test)
fi

if [[ -d "flutter" ]]; then
  echo "--> [flutter] dart test"
  (cd flutter && dart test)
fi

echo "==> All runtime tests passed."

# 3. 並行作業によるタグ衝突を回避しつつパッチバージョンを自動採番・Push
MAX_RETRIES=3
for ((i=1; i<=MAX_RETRIES; i++)); do
  git fetch --tags --force >/dev/null 2>&1 || true

  LATEST_TAG=$(git tag -l "v[0-9]*.[0-9]*.[0-9]*" --sort=-v:refname | head -n 1)

  if [[ -z "$LATEST_TAG" ]]; then
    NEW_TAG="v0.1.0"
  else
    VERSION="${LATEST_TAG#v}"
    IFS='.' read -r MAJOR MINOR PATCH <<< "$VERSION"
    NEW_TAG="v${MAJOR}.${MINOR}.$((PATCH + 1))"
  fi

  echo "==> Creating and pushing tag: ${NEW_TAG} (attempt ${i}/${MAX_RETRIES})"
  git tag -a "$NEW_TAG" -m "Release ${NEW_TAG}"

  if git push origin "$NEW_TAG" >/dev/null 2>&1; then
    echo ""
    echo "RELEASE_SUCCESS: ${NEW_TAG}"
    exit 0
  else
    echo "WARN: Tag ${NEW_TAG} conflicted on remote. Retrying..." >&2
    git tag -d "$NEW_TAG" >/dev/null 2>&1 || true
    sleep 1
  fi
done

echo "ERROR: ${MAX_RETRIES}回リトライしましたがタグの Push に失敗しました。" >&2
exit 1
```

---

## 3. 本体 Nx ワークスペース用：`scripts/sync-outside.sh`

指定されたランタイム（`go` / `next` / `angular` / `flutter` / `all`）とタグについて、`libs/share/outside/<target>` を完全に削除したうえで `git archive` によりクリーン展開します。

```bash
#!/usr/bin/env bash
set -euo pipefail

# Usage:
#   ./scripts/sync-outside.sh go v0.2.0
#   ./scripts/sync-outside.sh all v0.2.0

TARGET="${1:-}"
TAG="${2:-}"
OUTSIDE_REPO="${OUTSIDE_REPO:-../outside}"

if [[ -z "$TARGET" || -z "$TAG" ]]; then
  echo "Usage: $0 <go|next|angular|flutter|all> <tag>" >&2
  exit 1
fi

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_ROOT"

sync_one() {
  local lang="$1"
  local dest="libs/share/outside/${lang}"

  echo "==> Cleaning old directory: ${dest}"
  rm -rf "$dest"
  mkdir -p "$dest"

  echo "==> Extracting ${lang}/ at tag '${TAG}' from ${OUTSIDE_REPO}..."
  if [[ -d "$OUTSIDE_REPO" ]]; then
    git -C "$OUTSIDE_REPO" fetch --tags --force >/dev/null 2>&1 || true
    git -C "$OUTSIDE_REPO" archive --format=tar "$TAG" "$lang"       | tar -x -C "$dest" --strip-components=1
  else
    git archive --remote="$OUTSIDE_REPO" --format=tar "$TAG" "$lang"       | tar -x -C "$dest" --strip-components=1
  fi

  cat <<EOF > "${dest}/.outside-version"
tag=${TAG}
target=${lang}
synced_at=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
EOF

  echo "==> Synced ${dest} to ${TAG}"
}

if [[ "$TARGET" == "all" ]]; then
  for t in go next angular flutter; do
    sync_one "$t"
  done
else
  case "$TARGET" in
    go|next|angular|flutter)
      sync_one "$TARGET"
      ;;
    *)
      echo "ERROR: 不正なターゲットです: '${TARGET}' (go|next|angular|flutter|all)" >&2
      exit 1
      ;;
  esac
fi
```

---

## 4. セッション間 `relay` 運用サイクルまとめ

1. **本体セッション（開発中）:**
   OS プリミティブの不足を検知 → 業務背景を削ぎ落とした仕様書を作成し、`outside` 専用セッションへ `relay`。
2. **`outside` 専用セッション（門番）:**
   `08_agents_md_outside_gatekeeper.md` に照らし合わせて審査 → `Real` + `Fake` + 両面テストを実装 → `./scripts/release.sh` を実行 → 発行されたタグ名（例: `v0.2.0`）を返却。
3. **本体セッション（再開）:**
   `./scripts/sync-outside.sh all v0.2.0` を実行 → Nx キャッシュが更新されたクリーンな状態で実装・Unit テストを再開 → `npx nx affected -t lint,test` で完了検証。
