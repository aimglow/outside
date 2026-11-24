# 09. 運用 — 依頼・版の発行・取り込み・版の正本

`outside` の部品を 依頼して直し、版を出し、各 project が取り込むまでの運用を定める。
script の中身の正本は `outside` repo の `scripts/` (この文書に script を写さない)。部品の表 = `docs/developer/design.outside.yaml` の `components`。

---

## 1. 置き場

```text
outside repo (github.com/aimglow/outside、編集するのは outside 門番だけ)
├── go/ next/ angular/ flutter/                  # 口の部品 (runtime ごと)
├── lint-go/ lint-ts/ lint-dart/                 # 外を直に呼ぶ code を落とす lint の設定 (stack ごと)
├── lint-platform/                               # adapters 層の検査と雛形の作成
├── docs/deterministic-testing/                  # この文書群 (部品 docs)
└── scripts/
    ├── release.sh <部品> <major|minor|patch>    # 部品の test を回し 版の tag を打つ
    ├── sync-outside.sh <部品> <tag>             # 各 project が 部品を取り込む (各 project の scripts/ へ写して使う)
    └── test-scripts.sh                          # 上の 2 つの test

各 project
├── outside-platform.toml                        # project 固有の機能の語 (adapters.md §0)
├── scripts/sync-outside.sh                      # outside の scripts/sync-outside.sh の写し
├── docs/deterministic-testing/                  # 部品 docs の写し (.outside-version 付き)
└── libs/outside/<部品>/                         # 部品の写し (.outside-version 付き。編集しない)
```

---

## 2. 版

- 版は 部品ごとの tag `<部品>/vMAJOR.MINOR.PATCH`。部品ごとに独立して上がる (lint だけ上げる・go だけ上げる が できる)。
- 部品の形でない tag は作らない。`release.sh` は 部品の形でない tag が 1 つでも在れば 何もせず止まる。
- 段の決め方 = `design.outside.yaml` の `versioning` (0.x の間は 意味を変える変更も MINOR)。
- 古い版を使う project へ 直しだけを届ける時は その版の tag から branch `fix/<部品>-v<MAJOR>.<MINOR>` を切り PATCH の tag を打つ。

### 版の正本 (書き写さない)

| 知りたい事 | 正本 | 引き方 |
|---|---|---|
| 出ている版 | `outside` の tag | `git ls-remote --tags https://github.com/aimglow/outside.git '<部品>/v*'` |
| 各版で何が変わったか・利用する側がやる事 | `outside` の `CHANGELOG.md` | 部品の名前で引く |
| project が使っている版 | その project の `<置き場>/.outside-version` | `cat libs/outside/<部品>/.outside-version` |

版を memory・CLAUDE.md・issue・agent の定義に書き写さない (直し忘れた写しが 古い版を言い続けるため)。

---

## 3. 版の発行 (outside 門番)

1. 部品の dir を直し test を足す。commit する (作業木に未 commit の変更があると `release.sh` は止まる)。
2. `CHANGELOG.md` に 部品・版・変えた口や規則・利用する側がやる事 (MINOR = 次に触る時に上げてよい / MAJOR = 何を直してから上げるか) を書いて commit し main へ push する。
3. `./scripts/release.sh <部品> <major|minor|patch>` を実行する。部品の test を回し 通れば tag を打って push し `RELEASE_SUCCESS: <tag>` を出す。手で `git tag` を打たない。
4. 依頼元へ `RELEASE_SUCCESS: <tag>` を `ccark send` で返し 届いた事を確かめる。依頼元でない project の窓口と project-manager へも 同じ 1 通を送る。

---

## 4. 取り込み (各 project)

1. 初回だけ `outside` の `scripts/sync-outside.sh` を 自分の `scripts/` へ写す。
2. `./scripts/sync-outside.sh <部品> <tag>` を repo の根で実行する。一時の bare repo に その tag だけを取り (`git fetch --depth 1`)、`git archive` で 部品を取り出し 置き場を丸ごと置き換え `.outside-version` に tag を書く。誤り (他の部品の tag・形の違う tag・無い tag) の時は 置き場を変えずに止まる。GitHub は `git archive --remote` に応じないので この形を取る。
3. 置き場の写しは編集しない。部品の test は 置き場では回さない。
4. MAJOR は 各 project が 直す準備ができた時に その部品だけ上げる (全 project が同時に上げなくてよい)。

---

## 5. 依頼の流れ

1. **project の seat**: 口が足りない・規則で表せない物が出た → 業務の背景を除いた依頼 (app-rules.md §4 の書式) を outside 門番へ `ccark send`。
2. **outside 門番**: `reference.deterministic-testing.gatekeeper-rules.md` に照らして審査 → Real・Mock・両面の test を実装 → §3 で版を出す。
3. **project の seat**: §4 で取り込み 作業を再開 → `ccark prepush-gate` で完了を確かめる。
