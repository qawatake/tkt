package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	pushDir string
	dryRun  bool
)

var pushCmd = &cobra.Command{
	Use:   "push",
	Short: "ローカルでの編集差分をリモートのJIRAチケットに適用",
	Long: `ローカルでの編集差分をリモートのJIRAチケットに適用します。
ローカルにfetchしたものと差分があるファイルだけ更新します。
keyがないものはremoteにないチケットのため、JIRAにチケットを作成したあとにファイルのkeyを更新します。`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("ローカルの編集差分を %s からJIRAに適用します\n", pushDir)

		// ここでローカルの編集差分をJIRAに適用する処理を実装
		// 1. ~/.cache/gojira にリモートのチケットをfetch
		// 2. ローカルのチケットと比較して差分を検出
		// 3. 差分をJIRAに適用
		// 4. キャッシュを更新

		if dryRun {
			fmt.Println("ドライラン: 実際には適用されません")
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(pushCmd)

	// フラグの設定
	pushCmd.Flags().StringVarP(&pushDir, "dir", "d", "./tmp", "チケットディレクトリ")
	pushCmd.Flags().BoolVar(&dryRun, "dry-run", false, "実際に適用せずに差分のみ表示")
}
