# tkt拡張システム仕様書

このディレクトリには、tkt拡張システムの仕様と使用方法に関するドキュメントが含まれています。

## ドキュメント一覧

### [extension.md](./extension.md)
**拡張システムの完全ガイド**
- 基本的な使い方（`tkt extension list`、拡張の実行）
- 拡張システムの技術仕様（発見メカニズム、実行プロセス、エラーハンドリング）
- 拡張開発ガイド（bash/Go/Python例、必要条件）
- トラブルシューティング
- 実装詳細とセキュリティ考慮事項

## クイックスタート

### 拡張を使用する

1. 利用可能な拡張を確認：
   ```bash
   tkt extension list
   ```

2. 拡張を実行：
   ```bash
   tkt <extension-name> [args...]
   ```

### 拡張を開発する

1. 拡張ファイルを作成（`tkt-<name>`）
2. 実行権限を付与（`chmod +x`）
3. PATHに配置
4. テスト実行

## システム概要

tkt拡張システムは、GitHub CLI（`gh`）と同様の仕組みで動作します：

- **自動発見**: PATH内の`tkt-*`ファイルを自動発見
- **透明な実行**: `tkt <extension>`で拡張を実行
- **引数透過**: フラグや引数をそのまま拡張に転送
- **エラー処理**: 拡張のエラーを適切に処理

## アーキテクチャ

```
tkt command
├── Built-in commands (fetch, push, diff, ...)
├── Extension management (extension list)
└── Extension execution
    ├── PATH search (tkt-*)
    ├── Permission check
    └── Process execution
```

## 互換性

- **tktバージョン**: v1.0+
- **拡張バージョン**: 1.0
- **サポート言語**: bash, Go, Python, その他のPATH実行可能ファイル

## 関連リンク

- [tkt本体のドキュメント](../CLAUDE.md)
- [開発環境セットアップ](../README.md)
- [コントリビューションガイド](../CONTRIBUTING.md)