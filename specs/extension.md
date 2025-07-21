# extensionコマンド

`tkt extension` コマンドは、tkt拡張システムの管理機能を提供します。このシステムにより、ユーザーは`tkt-*`という名前の実行可能ファイルを作成することで、tktコマンドを拡張できます。

## 基本的な使い方

### 拡張の一覧表示

```bash
tkt extension list
# または短縮形
tkt ext list
```

現在システムで利用可能な拡張を表示します：

```
Found 3 extension(s):
  edit      /usr/local/bin/tkt-edit
  hello     /Users/user/bin/tkt-hello
  example   /tmp/tkt-example

Usage: tkt <extension-name> [args...]
```

### 拡張の実行

拡張は通常のtktサブコマンドと同じように実行できます：

```bash
# 基本的な実行
tkt hello

# 引数付きで実行
tkt hello world test

# フラグ付きで実行
tkt -v hello --flag value arg1 arg2
```

## 拡張システムの仕様

### 拡張の命名規則

- 拡張は`tkt-<name>`という名前でなければなりません
- `<name>`部分が拡張名として使用されます
- 例：`tkt-hello` → `tkt hello`で実行可能

### 拡張の発見メカニズム

#### 検索対象
- システムのPATH環境変数に含まれる全ディレクトリ
- `tkt-`で始まるファイル名のみ対象
- 実行可能ファイルのみ対象（実行権限が必要）

#### 優先順位
- PATHの順序に従って検索
- 同名の拡張が複数ある場合、最初に見つかったものが使用される
- 拡張名でアルファベット順にソート

### 拡張の実行

#### コマンド解析
1. `tkt <command> [args...]`の形式で実行
2. `<command>`が既存のサブコマンドかチェック
3. 既存のサブコマンドでない場合、拡張として実行を試行

#### 引数の渡し方
- フラグを含む全引数が拡張に渡される
- 拡張名自体は引数から除外される
- 例：`tkt -v hello --flag value arg1` → 拡張には`["-v", "--flag", "value", "arg1"]`が渡される

#### 環境
- stdin/stdout/stderrは親プロセスと共有
- 現在の作業ディレクトリを継承
- 環境変数を継承

### エラーハンドリング

#### 拡張が見つからない場合
- `extension '<name>' not found`エラーを出力
- 通常のCobraコマンドエラーハンドリングに戻る

#### 拡張実行時エラー
- 拡張の終了コードがそのまま返される
- 拡張のstderr出力がそのまま表示される

### 内蔵コマンドとの優先順位

1. 内蔵サブコマンド（fetch, push, diff等）が最優先
2. 内蔵サブコマンドでない場合のみ拡張を検索
3. フラグ（-v, --helpなど）は通常通り処理

## 拡張の開発

### 必要な条件

1. **ファイル名**: `tkt-<extension-name>`形式
2. **実行権限**: `chmod +x`で実行権限を付与
3. **PATH配置**: システムのPATH環境変数に含まれるディレクトリに配置
4. **shebang**: スクリプトの場合は適切なshebangを指定

### bash拡張の例

```bash
#!/bin/bash
# tkt-hello

show_help() {
    cat << EOF
Usage: tkt hello [options] [name]

A simple greeting extension for tkt.

Options:
    -h, --help      Show this help message
    -v, --verbose   Enable verbose output

Arguments:
    name            Name to greet (default: World)
EOF
}

# 引数解析
VERBOSE=false
NAME="World"

while [[ $# -gt 0 ]]; do
    case $1 in
        -h|--help)
            show_help
            exit 0
            ;;
        -v|--verbose)
            VERBOSE=true
            shift
            ;;
        -*)
            echo "Unknown option: $1" >&2
            exit 1
            ;;
        *)
            NAME="$1"
            shift
            ;;
    esac
done

# メイン処理
if [[ "$VERBOSE" == "true" ]]; then
    echo "Verbose mode enabled" >&2
fi

echo "Hello, $NAME!"
```

### Go拡張の例

```go
// tkt-info/main.go
package main

import (
    "flag"
    "fmt"
    "os"
)

func main() {
    var verbose bool
    flag.BoolVar(&verbose, "v", false, "Enable verbose output")
    flag.BoolVar(&verbose, "verbose", false, "Enable verbose output")
    flag.Parse()

    args := flag.Args()
    
    if verbose {
        fmt.Fprintf(os.Stderr, "Verbose mode enabled\n")
        fmt.Fprintf(os.Stderr, "Processing %d arguments\n", len(args))
    }

    fmt.Printf("tkt-info extension\n")
    fmt.Printf("Received %d arguments:\n", len(args))
    for i, arg := range args {
        fmt.Printf("  [%d]: %s\n", i, arg)
    }
}
```

ビルド：
```bash
go build -o tkt-info main.go
```

### Python拡張の例

```python
#!/usr/bin/env python3
# tkt-format

import argparse
import sys

def main():
    parser = argparse.ArgumentParser(
        prog='tkt format',
        description='Format files using tkt'
    )
    
    parser.add_argument('-v', '--verbose', action='store_true',
                       help='Enable verbose output')
    parser.add_argument('files', nargs='*', help='Files to format')
    
    args = parser.parse_args()
    
    if args.verbose:
        print(f"Processing {len(args.files)} files", file=sys.stderr)
    
    for filename in args.files:
        if args.verbose:
            print(f"Processing: {filename}", file=sys.stderr)
        print(f"Formatted: {filename}")

if __name__ == '__main__':
    main()
```

## トラブルシューティング

### 拡張が見つからない場合

1. ファイル名が`tkt-`で始まっているか確認
2. 実行権限があるか確認（`chmod +x tkt-name`）
3. PATH環境変数に含まれているか確認
4. `tkt extension list`で一覧に表示されるか確認

### 引数が正しく渡されない場合

`-v`フラグ付きで実行してデバッグ情報を確認：

```bash
tkt -v myext arg1 arg2
```

verbose.Enabledがtrueの場合、実行される拡張のパスと引数が表示されます。

## 実装詳細

### 主要コンポーネント

#### `internal/extension/extension.go`
- `Manager`: 拡張の発見と実行を管理
- `Extension`: 個別の拡張を表現
- `FindExtensions()`: PATH内の拡張を発見
- `Execute()`: 拡張を実行

#### `internal/cmd/root.go`
- カスタムコマンド解析ロジック
- 拡張実行の前処理

#### `internal/cmd/extension.go`
- `tkt extension list`コマンド
- 拡張管理機能

### セキュリティ考慮事項

#### 実行権限
- 実行可能ファイルのみが拡張として認識される
- ファイルの実行権限チェック（0111）を実施

#### PATH制限
- システムのPATH環境変数のみ参照
- 相対パスや特定ディレクトリへの特権的アクセスなし

#### 引数サニタイゼーション
- 引数はそのまま拡張に渡される
- 拡張側で適切な入力検証が必要