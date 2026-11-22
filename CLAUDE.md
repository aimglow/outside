# outside の規約 (CC セッション向け)

outside は 全 project が module として取り込む共通部品である。application とツールが 外
(時計・環境変数・file・git・子 process・作業場・通信・保存) を触る場所を この部品 1 つに限る。
設計の正本 = `docs/developer/design.outside.yaml`、テスト仕様 = `docs/developer/spec.outside.yaml`。

## git の流れ

- GitHub だけで運用する。ccark の deploy の流れ (便・gate・prepush-gate・cd-native) は使わない。
- 残る branch は main の 1 本。PR も CI も通さず main へ入れる。
- 編集は 作業用の branch を 別の作業木 (git worktree) に切って行う。main の作業木の中の file は 直に編集しない。
  手順 = `git worktree add ../w-<名前> -b work/<名前>` → 編集して commit →
  `git push origin HEAD:main` (早送りだけ。早送りできない時は main を取り込んで やり直す) →
  main の作業木で `git pull --ff-only`。作業用の branch は GitHub へ push しない。
- push の前に 手元で 全部の test と lint を回す。通らない物を push しない。
- 版は git の tag で切る (`vMAJOR.MINOR.PATCH`、Go の module は `go/vMAJOR.MINOR.PATCH`)。
  壊れた版を tag にしない。tag は動かさない (直しは 次の版)。
- commit の message は 何を・なぜ を 1 行目に書く。

## 使われ方

- 利用する project は この repo を module として取り込む (Go = module の版 / TypeScript・Dart = git の tag)。
- project の code は 外を直に呼ばず この部品の口 (port) を通す。
- 版は tag で固定して使う。main を追わない。全 project が 同じ版を使う。
- project の中に置いた この部品の写しは project の中で編集しない。直すのは この repo だけ。

## 部品の足し方

- 口や method が足りなくなったら 必要になった project が この repo に足す (project の中の写しには足さない)。
- 足す順 = 設計書 (ports の節) → テスト仕様 → 実装 (Go・TypeScript・Dart で 要る言語) → 本物と偽物の契約の test → 版を上げて tag。
- 足すだけの変更は MINOR (前の版を使う project を壊さない)。意味を変える・消す変更は MAJOR。

## 版が上がった時の伝え方

版の tag を切った者が 切ったその場で 次の 3 つを行う。3 つ済んで 版を切った と言う。

1. `CHANGELOG.md` に 1 項を足す: 版・日付・足した / 変えた 口と method・足した project・
   利用する project がやる事 (MINOR = 何もしなくてよい・次に触る時に上げる / MAJOR = 何を いつまでに直すか)。
2. 利用する全 project (ccark・amatelas・kifune・daikokuten・demaetei) の 窓口 (UHS) と project-manager へ
   同じ中身を 1 通ずつ送り 届いた事を確かめる (`ccark send`)。
3. 全 seat が起動時に受け取る指示 (ccark memory の `owner-outside-only`) の 現行の版の記述を 新しい版へ直す。

- MAJOR は 全 project が同時に上げる。切る前に 全 project の project-manager と 上げる時期を合わせる。
- MINOR は 前の版のままでも動く。ただし 版がずれた project は 定時の報告で名指され 次に触る時に揃える。
- project が 今どの版を使っているかは project の中の写しに置いた 版の記録 (1 行) で読む。

## 外への access と test の置き場

- 外 (実の時計・実の file system・実の git・実の子 process・実の通信) を触る code は
  この repo の 本物の実装 (real) の中だけに置く。
- real を 実物で確かめる test は この repo の中だけで行う。その test は 自分で作った物だけを触り
  機体に元から在る物 (機体の環境変数・/tmp に在る file・手元の repo) を読まない。
- 利用する project の test は 実物を触らない。この部品の偽物 (fake) を渡す。
- この部品自身の test は この repo の中で回す。利用する project の中では回さない
  (project に写すのは 実装と偽物だけ。project の検査は project の code を確かめる)。

## 偽物 (fake) の決め

- 口 1 つにつき 偽物は 1 つ。この repo が持つ。利用する project は 自分で偽物を作らない。
- 偽物は test が渡した値だけで振る舞いが決まる。実の時計・機体・file system を読まない。
- 本物と偽物は 同じ契約の test を通す (同じ入力に 同じ振る舞い)。偽物だけが通る・本物だけが通る
  振る舞いを 残さない。

## 作る順

設計書 → テスト仕様 → 実装 → test。順を飛ばさない。口の名前と意味は Go・TypeScript・Dart で同じにし
設計書の ports の節を 唯一の定義とする。

## 書かない物

- repo の外の絶対 path。
- system の一時置き場 (/tmp・os.TempDir・MkdirTemp("")) を前提にした処理。
- secret の値を 作業場へ写す処理。
- 文書の文を確かめる test。
