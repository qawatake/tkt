package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/qawatake/tkt/internal/config"
	"github.com/qawatake/tkt/internal/jira"
	"github.com/qawatake/tkt/internal/ticket"
	"github.com/qawatake/tkt/internal/verbose"
	"github.com/qawatake/tkt/pkg/utils"
	"github.com/spf13/cobra"
)

var pullCmd = &cobra.Command{
	Use:   "pull",
	Short: "JIRAチケットをダウンロードしてローカルにマージ",
	Long: `JIRAチケットをダウンロードして、ローカルディレクトリにマージします。
fetchとmergeコマンドを組み合わせたコマンドです。`,
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

		// 2. JIRAに接続
		jiraClient, err := jira.NewClient(cfg)
		if err != nil {
			return fmt.Errorf("JIRAクライアントの作成に失敗しました: %v", err)
		}

		// 3. チケットを取得（fetch部分）
		verbose.Println("JIRAからチケットを取得中...")
		tickets, err := jiraClient.FetchIssues()
		if err != nil {
			return fmt.Errorf("チケットの取得に失敗しました: %v", err)
		}

		verbose.Printf("%d 件のチケットを取得しました\n", len(tickets))

		// 4. キャッシュディレクトリを確保
		cacheDir, err := config.EnsureCacheDir()
		if err != nil {
			return fmt.Errorf("キャッシュディレクトリの作成に失敗しました: %v", err)
		}

		// 5. チケットをキャッシュに保存（fetch部分）
		savedCount := 0
		for _, ticket := range tickets {
			// キャッシュディレクトリに保存
			savedCachePath, err := ticket.SaveToFile(cacheDir)
			if err != nil {
				verbose.Printf("警告: チケット %s のキャッシュ保存に失敗しました: %v\n", ticket.Key, err)
			}

			verbose.Printf("保存: %s -> %s\n", ticket.Key, savedCachePath)
			savedCount++
		}

		verbose.Printf("\n%d 件のチケットを保存しました\n", savedCount)

		// 6. ローカルディレクトリにマージ（merge部分）
		verbose.Printf("JIRAチケットを %s にマージします\n", outputDir)

		// 出力ディレクトリを確保
		if err := utils.EnsureDir(outputDir); err != nil {
			return fmt.Errorf("出力ディレクトリの作成に失敗しました: %v", err)
		}

		// 7. -fフラグが設定されていない場合は差分を確認してユーザーに問い合わせ
		if !forceFlag {
			verbose.Println("ローカルとキャッシュの差分を検出中...")
			// キャッシュ→ローカルの差分を検出（mergeの場合は逆方向）
			diffs, err := ticket.CompareDirs(cacheDir, outputDir)
			if err != nil {
				return fmt.Errorf("差分の検出に失敗しました: %v", err)
			}

			// 差分があるチケットを抽出
			var changedTickets []ticket.DiffResult
			for _, diff := range diffs {
				if diff.HasDiff {
					changedTickets = append(changedTickets, diff)
				}
			}

			if len(changedTickets) > 0 {
				verbose.Printf("%d 件のファイルに差分があります\n", len(changedTickets))

				// ユーザーに確認を取る
				for _, diff := range changedTickets {
					fmt.Printf("\n=== ファイル: %s ===\n", filepath.Base(diff.FilePath))
					if diff.Key != "" {
						fmt.Printf("チケット: %s\n", diff.Key)
					}
					fmt.Printf("差分:\n%s\n", diff.DiffText)

					if !utils.PromptForConfirmation("このファイルを上書きしますか？") {
						fmt.Printf("スキップ: %s\n", filepath.Base(diff.FilePath))
						continue
					}

					// 確認されたファイルのみコピー
					srcPath := diff.FilePath
					dstPath := filepath.Join(outputDir, filepath.Base(diff.FilePath))
					if err := copyFile(srcPath, dstPath); err != nil {
						return fmt.Errorf("ファイルのコピーに失敗しました: %v", err)
					}
					verbose.Printf("コピー: %s -> %s\n", srcPath, dstPath)
				}

				verbose.Printf("キャッシュからローカルディレクトリへのマージが完了しました\n")
				return nil
			} else {
				verbose.Println("差分はありません")
				return nil
			}
		}

		// 8. -fフラグが設定されている場合は全ファイルを強制上書き
		entries, err := os.ReadDir(cacheDir)
		if err != nil {
			return err
		}

		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			srcPath := filepath.Join(cacheDir, entry.Name())
			dstPath := filepath.Join(outputDir, entry.Name())

			// ファイルをコピー
			if err := copyFile(srcPath, dstPath); err != nil {
				return fmt.Errorf("ファイルのコピーに失敗しました: %v", err)
			}
			verbose.Printf("コピー: %s -> %s\n", srcPath, dstPath)
		}

		verbose.Printf("キャッシュからローカルディレクトリへのマージが完了しました\n")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(pullCmd)

	pullCmd.Flags().StringVarP(&outputDir, "output", "o", "", "出力ディレクトリ")
	pullCmd.Flags().BoolVarP(&forceFlag, "force", "f", false, "既存ファイルを上書き")
}
