# TODO

- [x] total > maxResultsのときもすべてのデータを取得する。
- [x] fetchでキャッシュは問答無用で更新。
- [x] `merge`コマンドの実装。fetchでtmpは更新しない。
- [x] md -> yml -> jsonにしてduckdbで検索(できればUIでも)
  - https://github.com/qawatake/mdmdump
  - mdmdump tmp > .qwtk/hoge.json
  - `D select * from '.qwtk/hoge.json'`
- [ ] diffの書式改善。
- [ ] マークダウンの書式改善
- [ ] contextの利用。
- [ ] vscodeの拡張でスプリントごと、エピックごとにタスクを表示
- [ ] vscodeの拡張でタイトルをファイル名に
- [ ] vscodeの拡張でステータスをアイコンとして表示
- [ ] duckdb extensionの実装
