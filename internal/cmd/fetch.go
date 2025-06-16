package cmd

import (
	"fmt"

	"github.com/qawatake/tkt/internal/config"
	"github.com/qawatake/tkt/internal/jira"
	"github.com/qawatake/tkt/internal/ui"
	"github.com/qawatake/tkt/internal/verbose"
	"github.com/spf13/cobra"
)

var (
	outputDir string
)

var fetchCmd = &cobra.Command{
	Use:   "fetch",
	Short: "リモートのJIRAチケットの最新情報を取得します。",
	Long:  `リモートのJIRAチケットの最新情報を取得します。`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// 1. 設定ファイルを読み込む
		cfg, err := config.LoadConfig()
		if err != nil {
			return fmt.Errorf("設定ファイルの読み込みに失敗しました: %v", err)
		}

		// outputDirが指定されていない場合は設定ファイルのディレクトリを使用
		if outputDir == "" {
			if cfg.Directory == "" {
				return fmt.Errorf("設定ファイルにdirectoryが設定されていません。tkt initで設定してください")
			}
			outputDir = cfg.Directory
		}

		// 設定情報をデバッグ表示
		verbose.Printf("JIRA Server: %s\n", cfg.Server)
		verbose.Printf("Project Key: %s\n", cfg.Project.Key)
		verbose.Printf("Auth Type: %s\n", cfg.AuthType)
		if cfg.JQL != "" {
			verbose.Printf("Custom JQL: %s\n", cfg.JQL)
		}

		// チケット取得処理を一括実行
		savedCount, err := ui.WithSpinnerValue("チケット取得中...", func() (int, error) {
			// 2. JIRAに接続
			jiraClient, err := jira.NewClient(cfg)
			if err != nil {
				return 0, fmt.Errorf("JIRAクライアントの作成に失敗しました: %v", err)
			}

			// 3. チケットを取得
			tickets, err := jiraClient.FetchIssues()
			if err != nil {
				return 0, fmt.Errorf("チケットの取得に失敗しました: %v", err)
			}

			verbose.Printf("%d 件のチケットを取得しました\n", len(tickets))

			// 5. キャッシュディレクトリを確保
			cacheDir, err := config.ClearCacheDir()
			if err != nil {
				return 0, fmt.Errorf("キャッシュディレクトリの作成に失敗しました: %v", err)
			}

			// チケットを処理
			savedCount := 0
			for _, ticket := range tickets {
				// JIRAのイシューからTicketを作成

				// キャッシュディレクトリに保存
				savedCachePath, err := ticket.SaveToFile(cacheDir)
				if err != nil {
					verbose.Printf("警告: チケット %s のキャッシュ保存に失敗しました: %v\n", ticket.Key, err)
				}

				verbose.Printf("保存: %s -> %s\n", ticket.Key, savedCachePath)
				savedCount++
			}
			return savedCount, nil
		})
		if err != nil {
			return err
		}

		verbose.Printf("\n%d 件のチケットを保存しました\n", savedCount)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(fetchCmd)

	// フラグの設定
	fetchCmd.Flags().StringVarP(&outputDir, "output", "o", "", "出力ディレクトリ")
}
