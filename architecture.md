# gojira CLIアーキテクチャ設計

## 1. 概要

gojiraはJIRAチケットをローカルで編集し、それをJIRAに同期するためのCLIツールです。主な機能は以下の通りです：

- `fetch`: JIRAチケットをローカルにダウンロード
- `push`: ローカルでの編集差分をリモートのJIRAチケットに適用
- `diff`: ローカルとリモートのJIRAチケットの差分を表示

## 2. ディレクトリ構造

```
gojira/
├── cmd/
│   └── gojira/
│       └── main.go       # エントリーポイント
├── internal/
│   ├── config/           # 設定ファイル関連
│   │   ├── config.go     # 設定ファイル構造体
│   │   └── loader.go     # 設定ファイル読み込み
│   ├── jira/             # JIRA API関連
│   │   ├── client.go     # JIRAクライアント
│   │   ├── issue.go      # チケット操作
│   │   └── auth.go       # 認証関連
│   ├── cmd/              # コマンド実装
│   │   ├── root.go       # ルートコマンド
│   │   ├── fetch.go      # fetchコマンド
│   │   ├── push.go       # pushコマンド
│   │   └── diff.go       # diffコマンド
│   └── ticket/           # チケット操作関連
│       ├── parser.go     # マークダウンパーサー
│       ├── formatter.go  # マークダウンフォーマッター
│       └── diff.go       # 差分検出
└── pkg/
    ├── markdown/         # マークダウン操作ユーティリティ
    │   ├── frontmatter.go # フロントマター操作
    │   └── parser.go     # マークダウンパース
    └── utils/            # 汎用ユーティリティ
        ├── file.go       # ファイル操作
        └── time.go       # 時間操作
```

## 3. コマンド構造

### ルートコマンド

```
gojira - JIRAチケットローカル同期CLI
```

### サブコマンド

1. `fetch` - JIRAチケットをローカルにダウンロード
   - フラグ:
     - `--output, -o`: 出力ディレクトリ（デフォルト: `./tmp`）
     - `--force, -f`: 既存ファイルを上書き

2. `push` - ローカルでの編集差分をリモートのJIRAチケットに適用
   - フラグ:
     - `--dir, -d`: チケットディレクトリ（デフォルト: `./tmp`）
     - `--dry-run`: 実際に適用せずに差分のみ表示

3. `diff` - ローカルとリモートのJIRAチケットの差分を表示
   - フラグ:
     - `--dir, -d`: チケットディレクトリ（デフォルト: `./tmp`）
     - `--format, -f`: 出力フォーマット（text, json）

## 4. 設定ファイル管理

設定ファイルは `~/.config/gojira/config.yml` に保存され、以下の情報を含みます：

```yaml
auth_type: basic  # basic, bearer, mtls
login: username@example.com
server: https://example.atlassian.net
project:
  key: PROJECT
  type: next-gen
board:
  id: 1
  name: ボード名
  type: simple
epic:
  name: ""
  link: ""
issue:
  fields:
    custom:
      - name: Field1
        key: customfield_10001
        schema:
          datatype: string
  types:
    - id: "10001"
      name: タスク
      handle: Task
      subtask: false
jql: "project = PROJECT AND status = done"  # 同期対象のJQLクエリ
```

## 5. JIRAとの同期機能

### チケットのローカル表現

JIRAチケットは以下のフォーマットのマークダウンファイルとして保存されます：

```markdown
---
key: PROJECT-123
parentKey: PROJECT-100
type: task
status: To Do
assignee: username
reporter: reporter
created_at: 2023-01-01T12:00:00Z
updated_at: 2023-01-02T12:00:00Z
---

# チケットタイトル

チケット本文...
```

### 同期フロー

1. **fetch**:
   - JIRAからチケット情報を取得
   - マークダウンファイルに変換して保存
   - キャッシュとして `~/.cache/gojira` にも保存

2. **push**:
   - ローカルのマークダウンファイルを読み込み
   - キャッシュと比較して差分を検出
   - 差分をJIRAに適用
   - キャッシュを更新

3. **diff**:
   - ローカルのマークダウンファイルを読み込み
   - JIRAから最新情報を取得
   - 差分を検出して表示

## 6. エラーハンドリング

- 設定ファイルが存在しない場合は適切なエラーメッセージを表示
- JIRAとの通信エラーは詳細なエラーメッセージを表示
- ローカルファイルの読み書きエラーは適切に処理

## 7. 依存ライブラリ

- CLI: [spf13/cobra](https://github.com/spf13/cobra)
- 設定: [spf13/viper](https://github.com/spf13/viper)
- JIRA API: [andygrunwald/go-jira](https://github.com/andygrunwald/go-jira)
- マークダウン: [yuin/goldmark](https://github.com/yuin/goldmark)
- YAML/フロントマター: [go-yaml/yaml](https://github.com/go-yaml/yaml)
- 差分表示: [sergi/go-diff](https://github.com/sergi/go-diff)
