package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/qawatake/tkt/internal/config"
	"github.com/qawatake/tkt/internal/ticket"
	"github.com/qawatake/tkt/internal/verbose"
	"github.com/qawatake/tkt/pkg/utils"
	"github.com/spf13/cobra"
)

var (
	forceFlag bool
)

var mergeCmd = &cobra.Command{
	Use:   "merge",
	Short: "キャッシュにあるリモートのコピーでローカルのJIRAチケットを上書きします。",
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

		verbose.Printf("JIRAチケットを %s にマージします\n", outputDir)

		// 出力ディレクトリを確保
		if err := utils.EnsureDir(outputDir); err != nil {
			return fmt.Errorf("出力ディレクトリの作成に失敗しました: %v", err)
		}

		// 2. キャッシュディレクトリを確保
		cacheDir, err := config.EnsureCacheDir()
		if err != nil {
			return fmt.Errorf("キャッシュディレクトリの作成に失敗しました: %v", err)
		}

		// 3. -fフラグが設定されていない場合は差分を確認してユーザーに問い合わせ
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

		// 4. -fフラグが設定されている場合は全ファイルを強制上書き
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

// copyFile はファイルをコピーします
func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	return err
}

func init() {
	rootCmd.AddCommand(mergeCmd)

	mergeCmd.Flags().BoolVarP(&forceFlag, "force", "f", false, "既存ファイルを上書き")
}
