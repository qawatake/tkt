package cmd

import (
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "gojira",
	Short: "JIRAチケットローカル同期CLI",
	Long: `gojiraはJIRAチケットをローカルで編集し、それをJIRAに同期するCLIツールです。
主な機能は以下の通りです：
- fetch: JIRAチケットをローカルにダウンロード
- push: ローカルでの編集差分をリモートのJIRAチケットに適用
- diff: ローカルとリモートのJIRAチケットの差分を表示`,
}

// Execute executes the root command.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	// ここでグローバルフラグなどを設定する場合は追加
}
