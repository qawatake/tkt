package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/gojira/gojira/internal/config"
	"github.com/gojira/gojira/internal/verbose"
	"github.com/gojira/gojira/pkg/utils"
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
			outputDir = cfg.Directory
			if outputDir == "" {
				outputDir = "./tmp" // フォールバック
			}
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

		entries, err := os.ReadDir(cacheDir)
		if err != nil {
			return err
		} // outputDirにコピー
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			srcPath := filepath.Join(cacheDir, entry.Name())
			dstPath := filepath.Join(outputDir, entry.Name())

			// 既存ファイルがあり、上書きフラグが立っていない場合はスキップ
			if _, err := os.Stat(dstPath); err == nil && !forceFlag {
				verbose.Printf("スキップ: %s (既存ファイル)\n", dstPath)
				continue
			}
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
