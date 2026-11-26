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
| project が使っている版 | その project の `<置き場>/.outside-version` の 1 行目 | `head -1 libs/outside/<部品>/.outside-version` |

版を memory・CLAUDE.md・issue・agent の定義に書き写さない (直し忘れた写しが 古い版を言い続けるため)。

---

## 3. 版の発行 (outside 門番)

1. 部品の dir を直し test を足す。commit する (作業木に未 commit の変更があると `release.sh` は止まる)。
2. `CHANGELOG.md` に 部品・版・変えた口や規則・利用する側がやる事 (MINOR = 次に触る時に上げてよい / MAJOR = 何を直してから上げるか) を書いて commit し main へ push する。
3. `./scripts/release.sh <部品> <major|minor|patch>` を実行する。部品の test を回し 通れば tag を打って push し `RELEASE_SUCCESS: <tag>` を出す。手で `git tag` を打たない。
4. 依頼元へ `RELEASE_SUCCESS: <tag>` を `ccark send` で返し 届いた事を確かめる。依頼元でない project の窓口と project-manager へも 同じ 1 通を送る。

---

## 4. 取り込み (各 project)

1. `outside` の main の `scripts/sync-outside.sh` を 自分の `scripts/` へ写す。`CHANGELOG.md` に `scripts` の項が足されたら 写し直す。
2. `./scripts/sync-outside.sh <部品> <tag>` を repo の根で実行する。一時の bare repo に その tag だけを取り (`git fetch --depth 1`)、`git archive` で 部品を取り出し 置き場を丸ごと置き換え `.outside-version` に tag (1 行目) と 写した中身の指紋 (2 行目 = 全体、続く行 = file ごと) を書く。部品の test (Go の `*_test.go`・`testdata/`、TS の `*.test.*`・`*.spec.ts`、Dart の `*_test.dart`・`test/`) は写さない (部品の test は project で回さない)。誤り (他の部品の tag・形の違う tag・無い tag) の時は 置き場を変えずに止まる。GitHub は `git archive --remote` に応じないので この形を取る。
3. 置き場の写しは編集しない。部品の test は 置き場では回さない。
   写しが tag の中身のままかは `./scripts/sync-outside.sh --check <部品>` で確かめる (網に行かない。exit 0 = 一致 / 1 = 違う file を名指す / 3 = 判定不能 = 指紋の無い写し・置き場が無い)。land の前の段に 部品ごとに 1 段として置き 合流後の木で回す (古い branch の merge で写しが書き換わるのを止める)。
4. MAJOR は 各 project が 直す準備ができた時に その部品だけ上げる (全 project が同時に上げなくてよい)。

---

## 5. 依頼の流れ

1. **project の seat**: 口が足りない・規則で表せない物が出た → 業務の背景を除いた依頼 (app-rules.md §4 の書式) を outside 門番へ `ccark send`。
2. **outside 門番**: `reference.deterministic-testing.gatekeeper-rules.md` に照らして審査 → Real・Mock・両面の test を実装 → §3 で版を出す。
3. **project の seat**: §4 で取り込み 作業を再開 → `ccark prepush-gate` で完了を確かめる。

---

## 6. 各 project の付け替えの完了の条件

各 project は 業務の作業より先に 次の全部 (AND) を満たす。issue の DoD は この節を指して書く。
目的 = test の結果を 入力だけで決まる形にし cache を効かせて 同じ入力の test を二度回さない (build の slot の無駄な詰まりを減らす) こと と、壊れた code で必ず落ちる test だけを残して 品質を担保すること。

1. `outside` の部品を `scripts/sync-outside.sh` で repo の中に置いている (その project の stack の部品と docs)。置いた写しは編集しない。
2. 外を直に呼ぶ code が `libs/outside`・`libs/platform/*` の real・入口の `main` にしか無い = lint-go・lint-ts・lint-dart (その project の stack の分) の違反 0。Nx の `layer:` 境界の違反 0。
3. `libs/platform` が lint-platform の check に通る (push の前の検査の段として宣言)。依存先に固有の取り決めを読む物は lib、起こして終了コードを見るだけの物は `outside` の子 process の口。
4. test が ccark の `docs/conventions/test-writing.md` に合う: §6 の 4 条件 (段 `testguard-writing`・既存の違反の棚卸し・外の入力の宣言 (Nx の task の inputs を含む)・陽性対照) を満たし、app と tool の test は mock だけ、実物に触れる test は `libs/platform` の結合 test (tag `integration`) だけ、分類 C (test-classification.md §1) は消す。
5. 最終の確かめ = 独立の clone で `e2e/cache_test.txt` (無ければ作る) に 1 行足して commit し `ccark prepush-gate --full --keep-going` → 回る段は 組込の go test だけで その中で実行される package は 0。同じ commit の 2 回目 → 回った段 0。両方の log の行を issue に置く。回った段が残れば 段の名前と理由 (log の「段の指紋の再利用をしない [理由]」) を名指し、ccark の側の原因なら ccark の窓口へ送る。
6. 以上が その project の流れで main (または 宣言の land 先) に land している。

---

## 7. 取り込みの教訓 (次の基盤の設計と 次に取り込む project へ)

取り込みを済ませた project の完了の証跡から一般化した決め。基盤を足す時・取り込みを始める時に 先に読む。

1. **口の組を先に固め 版を 1.0.0 にしてから取り込む。** 最初の利用者と一緒に口を決めると 版が短い間に十数回動き 利用者は取り込み直しを繰り返す。口の組は 利用者が書き直す呼び出しを先に数え (lint の違反の一覧)、受け先の無い禁止を残さない形で揃えてから出す。
2. **docs と実物の一致を outside の test で止める。** docs を先に書くと 実物との食い違いが利用者の側で見つかる。docs に書く撃ち方・名前・語は outside の test (陽性対照つき) が確かめる物だけにする。
3. **push は区切りで 1 回、審査も 1 回。** 審査の記録は sha に結ばれるので push の度に振り出しへ戻る。作業を区切りまで手元で進め 1 回の push と 1 回の審査で通す。
4. **段を足す前に 3 問を問う。** ①何を確かめるか ②毎回要るか (入力が変わらなければ回らない形か) ③比べる相手を 直に git で読む前提は正しいか (段の出力を cache の鍵に入れない・比べる相手は 段の入力として宣言する)。
5. **land の段では 合流後の木で lint・testguard・写しの `--check` を回す。** 古い branch の merge で 取り決めや写しが消える・戻るのを止める。各段に 患部が在る状態で赤くなる陽性対照を付ける。
6. **outside と platform の置き場を 審査役に結ぶ。** `libs/outside/**`・`scripts/sync-outside.sh`・`outside-platform.toml`・`docs/deterministic-testing/**`・lint の設定を owner-paths で 審査役に結ぶ (取り決めの置き場の変更を 審査を経ずに入れない)。
7. **審査で差し戻す型。** 次を見たら差し戻す:
   - TOML の key の重複
   - mock が 知らない入力に 空で答える (V3 = 知らない入力は誤り)
   - 比べる相手を cache の鍵に入れない (R1 = 結果は source だけで決まる)
   - app の test が 本物の adapter を使う (lint-platform の tests が止める = enforcement.md §4.2)
   - 判定不能が FAIL を隠す (判定不能は 独立の区分で返す)
   - 段が 自分の要る物を 自分で用意しない (他の段の出力や 機体に元から在る物に頼る)
8. **合格の確かめ** = §6 の 5。1 回目に回る段が 組込の go test だけで その中の実行が 0、2 回目が 0 なら cache が効いている。

