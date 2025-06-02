package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/gojira/gojira/internal/config"
	"github.com/gojira/gojira/internal/jira"
	"github.com/gojira/gojira/pkg/utils"
	"github.com/spf13/cobra"
)

var (
	outputDir string
	forceFlag bool
)

var fetchCmd = &cobra.Command{
	Use:   "fetch",
	Short: "JIRAチケットをローカルにダウンロード",
	Long: `JIRAチケットをローカルにダウンロードします。
カレントディレクトリの指定されたディレクトリにJIRAチケットをダウンロードします。
エピックもissueチケットもフラットに格納します。`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("JIRAチケットを %s にダウンロードします\n", outputDir)

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

		// 4. マークダウンファイルに変換して保存
		// 出力ディレクトリを確保
		if err := utils.EnsureDir(outputDir); err != nil {
			return fmt.Errorf("出力ディレクトリの作成に失敗しました: %v", err)
		}

		// 5. キャッシュディレクトリを確保
		cacheDir, err := config.EnsureCacheDir()
		if err != nil {
			return fmt.Errorf("キャッシュディレクトリの作成に失敗しました: %v", err)
		}

		// チケットを処理
		savedCount := 0
		for _, ticket := range tickets {
			// JIRAのイシューからTicketを作成

			// 出力ファイルパスを決定
			fileName := ticket.Key + ".md"
			outputPath := filepath.Join(outputDir, fileName)

			// キャッシュディレクトリにも保存
			savedCachePath, err := ticket.SaveToFile(cacheDir)
			if err != nil {
				fmt.Printf("警告: チケット %s のキャッシュ保存に失敗しました: %v\n", ticket.Key, err)
			}

			// 既存ファイルの上書き確認
			if !forceFlag && utils.FileExists(outputPath) {
				fmt.Printf("ファイル %s は既に存在します。-f フラグで上書きできます。\n", outputPath)
				continue
			}

			// 出力ディレクトリに保存
			savedOutputPath, err := ticket.SaveToFile(outputDir)
			if err != nil {
				fmt.Printf("警告: チケット %s の保存に失敗しました: %v\n", ticket.Key, err)
				continue
			}

			fmt.Printf("保存: %s -> %s\n", ticket.Key, savedOutputPath)
			if savedCachePath != "" {
				fmt.Printf("キャッシュ: %s -> %s\n", ticket.Key, savedCachePath)
			}
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
	fetchCmd.Flags().BoolVarP(&forceFlag, "force", "f", false, "既存ファイルを上書き")
}
