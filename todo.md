# TODO

- [x] total > maxResultsのときもすべてのデータを取得する。
- [x] fetchでキャッシュは問答無用で更新。
- [x] `merge`コマンドの実装。fetchでtmpは更新しない。
- [x] md -> yml -> jsonにしてduckdbで検索(できればUIでも)
  - https://github.com/qawatake/mdmdump
  - mdmdump tmp > .qwtk/hoge.json
  - `D select * from '.qwtk/hoge.json'`
- [x] diffのフォーマットを改善する。
- [x] pushするときに、変更対象のファイルだけfetchしてきて内容を比較する
- [x] pushするときに、差分があったらdiffを表示して、pushするか確認する。
- [x] diffの書式改善。
- [x] 差分がないのにあると認識される。(pushしてもdiffが残る。)
  - item listの下に空行があるかないかjiraは気にする。
  - のに、fetchするとその空行がなくなる。
  - ので、adf→mdがおかしい。
- [x] fetchのパフォーマンス改善。
  - [x] 並列化
  - [x] フィールドのフィルタ
- [x] diffのpagerを改善。
- [x] コードブロックで言語がちゃんと読み込まれない。https://github.com/kentaro-m/blackfriday-confluence/tree/master を修正すればいい。
- [x] 書き込み対象外のフィールドを差分から除外する。
- [x] queryコマンド(duckdbとmdmdumpのラッパー)
- [x] errors k1low3のとか？
- [x] 設定ファイルの自動的な初期化
- [x] push後のcache更新
- [x] タイトルもfrontmatterに入れる。
- [x] `merge`でも差分を見ながら上書きするかどうかを選べるように。
- [x] tkt helpの修正。
- [x] キャッシュディレクトリの衝突回避。
- [x] queryやgrepの操作対象。
- [x] tkt grepを使った後にターミナルが壊れているっぽい。
- [ ] projectが1000コくらいあるケースもあるのでissueTypeを取りすぎるとtkt.ymlが巨大になる。（2万行くらい。）
- [ ] sprintを設定できるように。（要するにカスタムField対応？？）
- [ ] コードブロックの言語の対応。（例: `sh`、`bash`は非対応。`shell`は対応。）
- [ ] コードブロックの言語なし対応。（現状は`none`）
- [ ] 巨大すぎるtkt.yml。ただfetchしたデータをそのまま保存しているところはcacheに移動したい。
- [ ] `[ほげ] ふが`みたいなタイトルが文字化けする。(`[]`のあとに日本語があるとだめみたい。)
- [ ] 自動的なキャッシュ更新。最終更新時刻と`updated`の比較でいけそうな気がする。
- [ ] 図を足す。
- [ ] 全文検索（fzfだときつい？行ける？）
- [ ] sprintとかepicとか
- [ ] initでissue typeの情報を自動的に取得するように。
- [ ] チケット作成がうまくいかない。typeの翻訳がうまくいってないっぽい。
- [ ] チケット削除。
- [ ] マークダウンの書式改善
- [ ] contextの利用。
- [ ] 複数行の箇条書き
- [ ] チケットのキャッシュ(最終更新から一定時間はfetchしない)。
- [ ] vscodeの拡張でスプリントごと、エピックごとにタスクを表示
- [ ] vscodeの拡張でタイトルをファイル名に
- [ ] vscodeの拡張でステータスをアイコンとして表示
- [ ] duckdb extensionの実装
