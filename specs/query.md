# query

## 機能

`query` コマンドはローカルのチケットファイルをSQLで検索できます。

### 使用例

```bash
# REPLモード（インタラクティブ）
tkt query

# コマンドモード（JSON出力）
tkt query -c "select * from tickets"
tkt query -c "select key, summary, status from tickets where status = 'To Do'"
```

### オプション

- `-c, --command`: コマンドモード - SQLクエリを直接実行し、結果をJSON形式で出力
- `-w, --workspace`: ワークスペースディレクトリを検索対象にする
- `-d, --dir`: 検索対象ディレクトリを指定

## todo

1. [x] `-c "select * from tickets"`のようにreplではなくそのままコマンドを実行するモードを提供。さらに出力はjsonで。
