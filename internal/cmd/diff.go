package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	diffDir    string
	diffFormat string
)

var diffCmd = &cobra.Command{
	Use:   "diff",
	Short: "ローカルとリモートのJIRAチケットの差分を表示",
	Long: `ローカルで編集したJIRAチケットとリモートのJIRAチケットの差分を表示します。
差分を計算する前に~/.cache/gojiraにリモートのチケットをfetchします。`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("ローカルとリモートのJIRAチケットの差分を表示します（ディレクトリ: %s, フォーマット: %s）\n", diffDir, diffFormat)
		
		// ここでローカルとリモートのJIRAチケットの差分を表示する処理を実装
		// 1. ~/.cache/gojira にリモートのチケットをfetch
		// 2. ローカルのチケットと比較して差分を検出
		// 3. 差分を表示
		
		return nil
	},
}

func init() {
	rootCmd.AddCommand(diffCmd)
	
	// フラグの設定
	diffCmd.Flags().StringVarP(&diffDir, "dir", "d", "./tmp", "チケットディレクトリ")
	diffCmd.Flags().StringVarP(&diffFormat, "format", "f", "text", "出力フォーマット (text, json)")
}
