# outside の規約 (CC セッション向け)

outside は 全 project が module として取り込む共通部品である。application とツールが 外
(時計・環境変数・file・子 process・作業場・機体の値・ID・通信・保存) を触る場所を この部品 1 つに限る。
設計の正本 = `docs/developer/design.outside.yaml`、テスト仕様 = `docs/developer/spec.outside.yaml`。

## git の流れ

- GitHub だけで運用する。ccark の deploy の流れ (便・gate・prepush-gate・cd-native) は使わない。
- 残る branch は main の 1 本。PR も CI も通さず main へ入れる。
- 編集は 作業用の branch を 別の作業木 (git worktree) に切って行う。main の作業木の中の file は 直に編集しない。
  手順 = `git worktree add ../w-<名前> -b work/<名前>` → 編集して commit →
  `git push origin HEAD:main` (早送りだけ。早送りできない時は main を取り込んで やり直す) →
  main の作業木で `git pull --ff-only`。作業用の branch は GitHub へ push しない。
- push の前に 手元で 全部の test と lint を回す。通らない物を push しない。
- 版は 部品ごとの git の tag で切る (`<部品>/vMAJOR.MINOR.PATCH`、部品の表 = 設計書の components)。
  `./scripts/release.sh <部品> <major|minor|patch>` が その部品の test を回し 次の tag を打つ。壊れた版を tag にしない。tag は動かさない (直しは 次の版)。
- main は 常に 各部品の最新の版にしておく: 部品の dir を変える変更は その部品の版を上げて tag を切るまでを 1 つの作業として済ませ、
  tag の無い部品の変更を main に残さない。設計書など 部品でない文書の変更は 版を上げない。
- commit の message は 何を・なぜ を 1 行目に書く。

## 編集する者

- この repo を編集するのは 常駐の **outside 門番** (`outside-gatekeeper`、solviento) だけ。役の規約 = ccark の `agents/outside-gatekeeper.md`、
  門番規約 = `docs/deterministic-testing/reference.deterministic-testing.gatekeeper-rules.md`。
- 各 project の seat は この repo を直に編集しない。口や method が足りない時は 門番へ `ccark send` で依頼する
  (書式 = `docs/deterministic-testing/reference.deterministic-testing.app-rules.md` §4)。

## 使われ方 (取り込みの流れ)

正本 = `docs/deterministic-testing/` (`reference.deterministic-testing.md` と その part)。

1. project の seat が 門番へ依頼する。
2. 門番が 門番規約に照らして審査し、Real・Mock・両面の test を実装し、`./scripts/release.sh <部品> <major|minor|patch>` で test を回して tag を切る。
3. 門番が `RELEASE_SUCCESS: <tag>` を 依頼元へ `ccark send` で返す。
4. 依頼元が `./scripts/sync-outside.sh <部品> <tag>` で その tag を 一時の bare repo に取り `git archive` で 部品の dir を取り出し
   置き場 (設計書の components の place) を丸ごと置き換える (`.outside-version` に tag を記録する)。部品ごとに 版を選べる。
5. 依頼元は 置いた写しを編集しない。この部品の test は 置き場では回さない (Nx の project として test の target を持たせない)。

- 版は tag で固定して使う。main を追わない。全 project が 同じ版を使う。
- 外部依存の test は この repo (Real の自己検証) と 各 project の adapters 層 (`libs/platform/<機能>-<依存先>-<言語>`) の中だけ。

## 部品の足し方

- 口や method が足りなくなったら 必要になった project が この repo に足す (project の中に置いた物には足さない)。
- 足す順 = 設計書 (ports の節) → テスト仕様 → 実装 (Go・TypeScript・Dart で 要る言語) → 本物と mock の契約の test → 版を上げて tag。
- 版の番号の意味は 設計書の versioning.meaning が決める。0.x (どの project の main にも まだ入っていない間) は 意味を変える変更も MINOR。
  1.0.0 (最初の project の main に入る時) から: 足すだけの変更は MINOR (前の版を使う project を壊さない)。意味を変える・消す変更は MAJOR。

## 版が上がった時の伝え方

版の運用の正本 = `docs/deterministic-testing/reference.deterministic-testing.operations.md` (§2 版・§3 版の発行)。要点:

1. `CHANGELOG.md` に 部品・版・変えた口や規則・利用する側がやる事 を書く。
2. `./scripts/release.sh <部品> <段>` で tag を打つ (手で `git tag` を打たない。部品の形でない tag が在れば止まる)。
3. 依頼元と 全 project (ccark・amatelas・kifune・daikokuten・demaetei) の 窓口 (UHS) と project-manager へ 同じ中身を 1 通ずつ送り 届いた事を確かめる (`ccark send`)。

- 版を memory・CLAUDE.md・issue・agent の定義に書き写さない。版の正本 = tag (出ている版)・CHANGELOG.md (中身)・各 project の `.outside-version` (使っている版)。
- MAJOR は 各 project が 直す準備ができた時に その部品だけ上げる。

## 外への access と test の置き場

- 外 (実の時計・実の file system・実の git・実の子 process・実の通信) を触る code は
  この repo の 本物の実装 (real) の中だけに置く。
- real を 実物で確かめる test は この repo の中だけで行う。その test は 自分で作った物だけを触り
  機体に元から在る物 (機体の環境変数・/tmp に在る file・手元の repo) を読まない。
- 利用する project の test は 実物を触らない。この部品の mock を渡す。
- この部品自身の test は この repo の中で回す。利用する project の中では回さない
  (置き場 `libs/outside/<部品>/` は 各 project の test・lint の対象から外す = enforcement.md §2〜§4)。

## mock の決め

mock = test にとって有用で妥当な 決めた値を返す 口の実装。test が渡した値だけで振る舞いが決まり
本物と同じ契約を満たす。中身を問わない代用品のことではない。
この repo と 利用する project の 文書・code・issue は mock と書く。

- 口 1 つにつき mock は 1 つ。この repo が持つ。利用する project は 自分で mock を作らない。
- mock は test が渡した値だけで振る舞いが決まる。実の時計・機体・file system を読まない。
- 本物と mock は 同じ契約の test を通す (同じ入力に 同じ振る舞い)。 mock だけが通る・本物だけが通る
  振る舞いを 残さない。

## 作る順

設計書 → テスト仕様 → 実装 → test。順を飛ばさない。口の名前と意味は Go・TypeScript・Dart で同じにし
設計書の ports の節を 唯一の定義とする。

## 書かない物

- repo の外の絶対 path。
- system の一時置き場 (/tmp・os.TempDir・MkdirTemp("")) を前提にした処理。
- secret の値を 作業場へ複製する処理。
- 文書の文を確かめる test。
