package cmd

import (
	"fmt"

	"github.com/gojira/gojira/internal/config"
	"github.com/gojira/gojira/internal/jira"
	"github.com/gojira/gojira/internal/ticket"
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

		// 1. 設定ファイルを読み込む
		cfg, err := config.LoadConfig()
		if err != nil {
			return fmt.Errorf("設定ファイルの読み込みに失敗しました: %v", err)
		}

		// 2. キャッシュディレクトリを確保
		cacheDir, err := config.EnsureCacheDir()
		if err != nil {
			return fmt.Errorf("キャッシュディレクトリの作成に失敗しました: %v", err)
		}

		// 3. JIRAに接続してリモートのチケットをキャッシュにfetch
		fmt.Println("リモートのJIRAチケットをキャッシュに取得中...")
		jiraClient, err := jira.NewClient(cfg)
		if err != nil {
			return fmt.Errorf("JIRAクライアントの作成に失敗しました: %v", err)
		}

		// リモートのチケットを取得
		tickets, err := jiraClient.FetchIssues()
		if err != nil {
			return fmt.Errorf("リモートチケットの取得に失敗しました: %v", err)
		}

		// キャッシュディレクトリに保存
		fmt.Printf("リモートから %d 件のチケットを取得しました\n", len(tickets))
		for _, ticket := range tickets {
			_, err := ticket.SaveToFile(cacheDir)
			if err != nil {
				fmt.Printf("警告: チケット %s のキャッシュ保存に失敗しました: %v\n", ticket.Key, err)
			}
		}

		// 4. ローカルとキャッシュの差分を検出
		fmt.Println("ローカルとリモートの差分を検出中...")
		diffs, err := ticket.CompareDirs(pushDir, cacheDir)
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

		if len(changedTickets) == 0 {
			fmt.Println("差分はありません")
			return nil
		}

		fmt.Printf("%d 件のチケットに差分があります\n", len(changedTickets))

		// 5. 差分をJIRAに適用
		if dryRun {
			fmt.Println("ドライラン: 実際には適用されません")
			for _, diff := range changedTickets {
				fmt.Printf("\n--- %s ---\n", diff.Key)
				fmt.Println(diff.DiffText)
			}
			return nil
		}

		// 実際に適用
		updatedCount := 0
		createdCount := 0
		for _, diff := range changedTickets {
			// ローカルのチケットを読み込み
			localTicket, err := ticket.FromFile(diff.FilePath)
			if err != nil {
				fmt.Printf("警告: チケット %s の読み込みに失敗しました: %v\n", diff.Key, err)
				continue
			}

			if localTicket.Key == "" {
				// 新規チケット作成
				fmt.Printf("新規チケットを作成中: %s\n", localTicket.Title)

				// JIRAにチケットを作成
				newIssue, err := jiraClient.CreateIssue(localTicket.Type, localTicket.Title, localTicket.Body, localTicket.ParentKey)
				if err != nil {
					fmt.Printf("エラー: チケット作成に失敗しました: %v\n", err)
					continue
				}

				// ローカルファイルのKeyを更新
				localTicket.Key = newIssue.Key
				_, err = localTicket.SaveToFile(pushDir)
				if err != nil {
					fmt.Printf("警告: ローカルファイルの更新に失敗しました: %v\n", err)
				}

				// キャッシュも更新
				remoteTicket := ticket.FromIssue(newIssue)
				_, err = remoteTicket.SaveToFile(cacheDir)
				if err != nil {
					fmt.Printf("警告: キャッシュの更新に失敗しました: %v\n", err)
				}

				fmt.Printf("作成完了: %s\n", newIssue.Key)
				createdCount++
			} else {
				// 既存チケット更新
				fmt.Printf("チケットを更新中: %s\n", localTicket.Key)

				// JIRAを更新
				err := jiraClient.UpdateIssue(*localTicket)
				if err != nil {
					fmt.Printf("エラー: チケット更新に失敗しました: %v\n", err)
					continue
				}

				// キャッシュを更新
				_, err = localTicket.SaveToFile(cacheDir)
				if err != nil {
					fmt.Printf("警告: キャッシュの更新に失敗しました: %v\n", err)
				}

				fmt.Printf("更新完了: %s\n", localTicket.Key)
				updatedCount++
			}
		}

		fmt.Printf("\n完了: %d 件作成, %d 件更新\n", createdCount, updatedCount)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(pushCmd)

	// フラグの設定
	pushCmd.Flags().StringVarP(&pushDir, "dir", "d", "./tmp", "チケットディレクトリ")
	pushCmd.Flags().BoolVar(&dryRun, "dry-run", false, "実際に適用せずに差分のみ表示")
}
