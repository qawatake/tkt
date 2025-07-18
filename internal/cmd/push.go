package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/qawatake/tkt/internal/config"
	"github.com/qawatake/tkt/internal/jira"
	"github.com/qawatake/tkt/internal/pkg/utils"
	"github.com/qawatake/tkt/internal/ticket"
	"github.com/qawatake/tkt/internal/ui"
	"github.com/qawatake/tkt/internal/verbose"
	"github.com/sourcegraph/conc/pool"
	"github.com/spf13/cobra"
)

var (
	pushDir string
	dryRun  bool
	force   bool
)

var pushCmd = &cobra.Command{
	Use:   "push",
	Short: "ローカルでの編集差分をリモートのJIRAチケットに適用します。",
	Long: `ローカルでの編集差分をリモートのJIRAチケットに適用します。
keyがチケットはリモートにないチケットのため、JIRAにチケットを作成したあとにファイルのkeyを更新します。

-f, --force フラグを使用すると、確認なしで強制的にpushされます。`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// 1. 設定ファイルを読み込む
		cfg, err := config.LoadConfig()
		if err != nil {
			return fmt.Errorf("設定ファイルの読み込みに失敗しました: %v", err)
		}

		// pushDirが指定されていない場合は設定ファイルのディレクトリを使用
		if pushDir == "" {
			if cfg.Directory == "" {
				return fmt.Errorf("設定ファイルにdirectoryが設定されていません。tkt initで設定してください")
			}
			pushDir = cfg.Directory
		}

		verbose.Printf("ローカルの編集差分を %s からJIRAに適用します\n", pushDir)

		// 差分検出処理を一括実行
		type diffResult struct {
			changedTickets []ticket.DiffResult
			jiraClient     *jira.Client
		}

		result, err := ui.WithSpinnerValue("差分を検出中...", func() (diffResult, error) {
			// 2. キャッシュディレクトリを確保
			cacheDir, err := config.EnsureCacheDir()
			if err != nil {
				return diffResult{}, fmt.Errorf("キャッシュディレクトリの作成に失敗しました: %v", err)
			}

			// 3. JIRAに接続してリモートのチケットをキャッシュにfetch
			jiraClient, err := jira.NewClient(cfg)
			if err != nil {
				return diffResult{}, fmt.Errorf("JIRAクライアントの作成に失敗しました: %v", err)
			}

			// 4. ローカルとキャッシュの差分を検出
			diffs, err := ticket.CompareDirs(pushDir, cacheDir)
			if err != nil {
				return diffResult{}, fmt.Errorf("差分の検出に失敗しました: %v", err)
			}

			// 差分があるチケットを抽出
			var changedTickets []ticket.DiffResult
			for _, diff := range diffs {
				if diff.HasDiff {
					changedTickets = append(changedTickets, diff)
				}
			}

			if len(changedTickets) == 0 {
				return diffResult{changedTickets: changedTickets, jiraClient: jiraClient}, nil
			}

			// 差分があるチケットについては最新の状態をキャッシュに保存し直す。
			// 新規作成以外のキーを収集
			var keysToFetch []string
			for _, diff := range changedTickets {
				if diff.Key != "" {
					keysToFetch = append(keysToFetch, diff.Key)
				}
			}

			// Bulk Fetch APIを使って一括取得
			if len(keysToFetch) > 0 {
				remoteTickets, err := jiraClient.BulkFetchIssues(keysToFetch)
				if err != nil {
					return diffResult{}, err
				}

				// 取得したチケットをキャッシュに保存
				for _, remoteTicket := range remoteTickets {
					_, err = remoteTicket.SaveToFile(cacheDir)
					if err != nil {
						return diffResult{}, err
					}
				}
			}

			// 改めて差分を検出
			diffs, err = ticket.CompareDirs(pushDir, cacheDir)
			if err != nil {
				return diffResult{}, fmt.Errorf("差分の検出に失敗しました: %v", err)
			}

			// 差分があるチケットを抽出
			changedTickets = nil
			for _, diff := range diffs {
				if diff.HasDiff {
					changedTickets = append(changedTickets, diff)
				}
			}

			return diffResult{changedTickets: changedTickets, jiraClient: jiraClient}, nil
		})
		if err != nil {
			return err
		}

		changedTickets := result.changedTickets
		jiraClient := result.jiraClient

		if len(changedTickets) == 0 {
			verbose.Println("差分はありません")
			return nil
		}

		verbose.Printf("%d 件のチケットに差分があります\n", len(changedTickets))

		if force {
			verbose.Println("フォースモード: 確認なしで全てのファイルをpushします")
		}

		// 5. 差分をJIRAに適用
		if dryRun {
			verbose.Println("ドライラン: 実際には適用されません")
			for _, diff := range changedTickets {
				verbose.Printf("\n--- %s ---\n", diff.Key)
				verbose.Println(diff.DiffText)
			}
			return nil
		}

		// ユーザーに確認を取る
		var confirmedTickets []ticket.DiffResult
		for _, diff := range changedTickets {
			if !dryRun && !force {
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
			confirmedTickets = append(confirmedTickets, diff)
		}

		if len(confirmedTickets) == 0 {
			verbose.Println("適用するチケットがありません")
			return nil
		}

		// 実際に適用（conc poolを使用して最大5並列で処理）
		var updatedCount, createdCount, deletedCount int
		var mu sync.Mutex

		err = ui.WithSpinner("変更を適用中...", func() error {
			// キャッシュディレクトリを再取得
			cacheDir, err := config.EnsureCacheDir()
			if err != nil {
				return fmt.Errorf("キャッシュディレクトリの作成に失敗しました: %v", err)
			}

			p := pool.New().WithMaxGoroutines(5).WithErrors()
			for _, diff := range confirmedTickets {
				p.Go(func() error {
					// 削除されたチケットかどうかをチェック
					if strings.HasPrefix(filepath.Base(diff.FilePath), ".") {
						// 削除されたチケットの処理
						localTicket, err := ticket.FromFile(diff.FilePath)
						if err != nil {
							return fmt.Errorf("削除対象チケット %s の読み込みに失敗しました: %v", diff.Key, err)
						}

						verbose.Printf("チケットを削除中: %s\n", localTicket.Key)

						// JIRAからチケットを削除
						err = jiraClient.DeleteIssue(localTicket.Key)
						if err != nil {
							return fmt.Errorf("チケット削除に失敗しました: %v", err)
						}

						// 削除マークファイル（ドットプレフィックス）を削除
						err = os.Remove(diff.FilePath)
						if err != nil {
							verbose.Printf("警告: 削除マークファイル %s の削除に失敗しました: %v\n", diff.FilePath, err)
						}

						// キャッシュからも削除
						originalFileName := filepath.Base(diff.FilePath)[1:] // .PRJ-123.md -> PRJ-123.md
						cacheFile := filepath.Join(cacheDir, originalFileName)
						err = os.Remove(cacheFile)
						if err != nil && !os.IsNotExist(err) {
							verbose.Printf("警告: キャッシュファイル %s の削除に失敗しました: %v\n", cacheFile, err)
						}

						verbose.Printf("削除完了: %s\n", localTicket.Key)
						mu.Lock()
						deletedCount++
						mu.Unlock()
						return nil
					}

					localTicket, err := ticket.FromFile(diff.FilePath)
					if err != nil {
						return fmt.Errorf("チケット %s の読み込みに失敗しました: %v", diff.Key, err)
					}

					if localTicket.Key == "" {
						// 新規チケット作成
						verbose.Printf("新規チケットを作成中: %s\n", localTicket.Title)

						// JIRAにチケットを作成
						createdTicket, err := jiraClient.CreateIssue(localTicket)
						if err != nil {
							return fmt.Errorf("チケット作成に失敗しました: %v", err)
						}

						// 元のファイルパスを保存
						originalFilePath := diff.FilePath

						// ローカルファイルのKeyを更新
						localTicket.Key = createdTicket.Key
						newFilePath, err := localTicket.SaveToFile(pushDir)
						if err != nil {
							return fmt.Errorf("ローカルファイルの更新に失敗しました: %v", err)
						}

						// 元のファイルを削除（新しいファイルパスと異なる場合のみ）
						if originalFilePath != newFilePath {
							err = os.Remove(originalFilePath)
							if err != nil {
								verbose.Printf("警告: 元のファイル %s の削除に失敗しました: %v\n", originalFilePath, err)
							} else {
								verbose.Printf("元のファイル %s を削除し、%s にリネームしました\n", originalFilePath, newFilePath)
							}
						}

						// キャッシュも更新（CreateIssueが既に正しいフォーマットで返すため直接保存）
						_, err = createdTicket.SaveToFile(cacheDir)
						if err != nil {
							return fmt.Errorf("キャッシュの更新に失敗しました: %v", err)
						}

						verbose.Printf("作成完了: %s\n", createdTicket.Key)
						mu.Lock()
						createdCount++
						mu.Unlock()
					} else {
						// 既存チケット更新
						verbose.Printf("チケットを更新中: %s\n", localTicket.Key)

						// JIRAを更新
						err := jiraClient.UpdateIssue(*localTicket)
						if err != nil {
							return fmt.Errorf("チケット更新に失敗しました: %v", err)
						}

						// キャッシュを更新（pushが成功したので最新の状態をキャッシュに保存）
						// ローカルチケットをそのまま使わずにremoteからfetchする理由：
						// - JIRAが自動更新する項目（updated日時、version等）を確実に取得
						// - 権限やvalidationでJIRA側で値が変更される可能性への対応
						// - データフロー（fetch→cache）の一貫性維持
						remoteTicket, err := jiraClient.FetchIssue(localTicket.Key)
						if err != nil {
							return fmt.Errorf("更新後のチケット取得に失敗しました: %v", err)
						}
						_, err = remoteTicket.SaveToFile(cacheDir)
						if err != nil {
							return fmt.Errorf("キャッシュの更新に失敗しました: %v", err)
						}

						verbose.Printf("更新完了: %s\n", localTicket.Key)
						mu.Lock()
						updatedCount++
						mu.Unlock()
					}
					return nil
				})
			}
			return p.Wait()
		})
		if err != nil {
			fmt.Printf("以下のエラーが発生しました:\n%v\n", err)
			fmt.Printf("成功した分: %d 件作成, %d 件更新, %d 件削除\n", createdCount, updatedCount, deletedCount)
			return fmt.Errorf("一部の処理でエラーが発生しました")
		}

		verbose.Printf("\n完了: %d 件作成, %d 件更新, %d 件削除\n", createdCount, updatedCount, deletedCount)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(pushCmd)

	// フラグの設定
	pushCmd.Flags().StringVarP(&pushDir, "dir", "d", "", "チケットディレクトリ")
	pushCmd.Flags().BoolVar(&dryRun, "dry-run", false, "実際に適用せずに差分のみ表示")
	pushCmd.Flags().BoolVarP(&force, "force", "f", false, "確認なしで強制的にpush")
}
