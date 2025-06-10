# fetchコマンド

`tkt fetch` コマンドは、JQLクエリを使用してJIRAサーバーからチケットを取得し、キャッシュディレクトリ（`~/.cache/tkt/`）に保存します。このファイルはremoteのチケットのコピーであり、これをもとにpushやdiffでの差分検出に使用されます。

## todo

1. [x] タイトルもfrontmatterに追加する。
2. [ ] urlをfrontmatterに追加する。
