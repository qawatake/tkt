package cmd

import (
	"fmt"

	"github.com/gojira/gojira/internal/config"
	"github.com/gojira/gojira/internal/jira"
	"github.com/gojira/gojira/internal/ticket"
	"github.com/gojira/gojira/internal/verbose"
	"github.com/gojira/gojira/pkg/utils"
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
		// 1. 設定ファイルを読み込む
		cfg, err := config.LoadConfig()
		if err != nil {
			return fmt.Errorf("設定ファイルの読み込みに失敗しました: %v", err)
		}

		// pushDirが指定されていない場合は設定ファイルのディレクトリを使用
		if pushDir == "" {
			pushDir = cfg.Directory
			if pushDir == "" {
				pushDir = "./tmp" // フォールバック
			}
		}

		verbose.Printf("ローカルの編集差分を %s からJIRAに適用します\n", pushDir)

		// 2. キャッシュディレクトリを確保
		cacheDir, err := config.EnsureCacheDir()
		if err != nil {
			return fmt.Errorf("キャッシュディレクトリの作成に失敗しました: %v", err)
		}

		// 3. JIRAに接続してリモートのチケットをキャッシュにfetch
		verbose.Println("リモートのJIRAチケットをキャッシュに取得中...")
		jiraClient, err := jira.NewClient(cfg)
		if err != nil {
			return fmt.Errorf("JIRAクライアントの作成に失敗しました: %v", err)
		}

		// 4. ローカルとキャッシュの差分を検出
		verbose.Println("ローカルとリモートの差分を検出中...")
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
			verbose.Println("差分はありません")
			return nil
		}

		// 差分があるチケットについては最新の状態をキャッシュに保存し直す。
		for _, diff := range changedTickets {
			key := diff.Key
			if key == "" {
				// 新規作成なのでスキップ
				continue
			}
			remoteTicket, err := jiraClient.FetchIssue(key)
			if err != nil {
				return err
			}
			_, err = remoteTicket.SaveToFile(cacheDir)
			if err != nil {
				return err
			}
		}

		// 改めて差分を検出
		diffs, err = ticket.CompareDirs(pushDir, cacheDir)
		if err != nil {
			return fmt.Errorf("差分の検出に失敗しました: %v", err)
		}

		// 差分があるチケットを抽出
		changedTickets = nil
		for _, diff := range diffs {
			if diff.HasDiff {
				changedTickets = append(changedTickets, diff)
			}
		}

		verbose.Printf("%d 件のチケットに差分があります\n", len(changedTickets))

		// 5. 差分をJIRAに適用
		if dryRun {
			verbose.Println("ドライラン: 実際には適用されません")
			for _, diff := range changedTickets {
				verbose.Printf("\n--- %s ---\n", diff.Key)
				verbose.Println(diff.DiffText)
			}
			return nil
		}

		// 実際に適用
		updatedCount := 0
		createdCount := 0
		for _, diff := range changedTickets {
			// 差分がある場合はユーザに確認
			if !dryRun {
				fmt.Printf("\n=== ファイル: %s ===\n", diff.FilePath)
				if diff.Key != "" {
					fmt.Printf("チケット: %s\n", diff.Key)
				} else {
					fmt.Printf("新規チケット\n")
				}
				fmt.Printf("差分:\n%s\n", diff.DiffText)

				if !utils.PromptForConfirmation("このファイルをpushしますか？") {
					fmt.Printf("スキップ: %s\n", diff.FilePath)
					continue
				}
			}

			// ローカルのチケットを読み込み
			localTicket, err := ticket.FromFile(diff.FilePath)
			if err != nil {
				verbose.Printf("警告: チケット %s の読み込みに失敗しました: %v\n", diff.Key, err)
				continue
			}

			if localTicket.Key == "" {
				// 新規チケット作成
				verbose.Printf("新規チケットを作成中: %s\n", localTicket.Title)

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

				verbose.Printf("作成完了: %s\n", newIssue.Key)
				createdCount++
			} else {
				// 既存チケット更新
				verbose.Printf("チケットを更新中: %s\n", localTicket.Key)

				// JIRAを更新
				err := jiraClient.UpdateIssue(*localTicket)
				if err != nil {
					fmt.Printf("エラー: チケット更新に失敗しました: %v\n", err)
					continue
				}

				verbose.Printf("更新完了: %s\n", localTicket.Key)
				updatedCount++
			}
		}

		verbose.Printf("\n完了: %d 件作成, %d 件更新\n", createdCount, updatedCount)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(pushCmd)

	// フラグの設定
	pushCmd.Flags().StringVarP(&pushDir, "dir", "d", "", "チケットディレクトリ")
	pushCmd.Flags().BoolVar(&dryRun, "dry-run", false, "実際に適用せずに差分のみ表示")
}
