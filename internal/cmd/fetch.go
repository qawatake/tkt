package cmd

import (
	"fmt"

	"github.com/gojira/gojira/internal/config"
	"github.com/gojira/gojira/internal/jira"
	"github.com/spf13/cobra"
)

var (
	outputDir string
)

var fetchCmd = &cobra.Command{
	Use:   "fetch",
	Short: "JIRAチケットをローカルにダウンロード",
	Long: `JIRAチケットをローカルにダウンロードします。
カレントディレクトリの指定されたディレクトリにJIRAチケットをダウンロードします。
エピックもissueチケットもフラットに格納します。`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// 1. 設定ファイルを読み込む
		cfg, err := config.LoadConfig()
		if err != nil {
			return fmt.Errorf("設定ファイルの読み込みに失敗しました: %v", err)
		}

		// 設定情報をデバッグ表示
		fmt.Printf("JIRA Server: %s\n", cfg.Server)
		fmt.Printf("Project Key: %s\n", cfg.Project.Key)
		fmt.Printf("Auth Type: %s\n", cfg.AuthType)
		if cfg.JQL != "" {
			fmt.Printf("Custom JQL: %s\n", cfg.JQL)
		}

		// 2. JIRAに接続
		jiraClient, err := jira.NewClient(cfg)
		if err != nil {
			return fmt.Errorf("JIRAクライアントの作成に失敗しました: %v", err)
		}

		// 3. チケットを取得
		fmt.Println("JIRAからチケットを取得中...")
		tickets, err := jiraClient.FetchIssues()
		if err != nil {
			return fmt.Errorf("チケットの取得に失敗しました: %v", err)
		}

		fmt.Printf("%d 件のチケットを取得しました\n", len(tickets))

		// 5. キャッシュディレクトリを確保
		cacheDir, err := config.EnsureCacheDir()
		if err != nil {
			return fmt.Errorf("キャッシュディレクトリの作成に失敗しました: %v", err)
		}

		// チケットを処理
		savedCount := 0
		for _, ticket := range tickets {
			// JIRAのイシューからTicketを作成

			// キャッシュディレクトリに保存
			savedCachePath, err := ticket.SaveToFile(cacheDir)
			if err != nil {
				fmt.Printf("警告: チケット %s のキャッシュ保存に失敗しました: %v\n", ticket.Key, err)
			}

			fmt.Printf("保存: %s -> %s\n", ticket.Key, savedCachePath)
			savedCount++
		}

		fmt.Printf("\n%d 件のチケットを保存しました\n", savedCount)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(fetchCmd)

	// フラグの設定
	fetchCmd.Flags().StringVarP(&outputDir, "output", "o", "./tmp", "出力ディレクトリ")
}
