package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/gojira/gojira/internal/config"
	"github.com/gojira/gojira/internal/ticket"
	"github.com/spf13/cobra"
)

var (
	diffDir    string
	diffFormat string
)

var diffCmd = &cobra.Command{
	Use:   "diff",
	Short: "ローカルとリモートのJIRAチケットの差分を表示",
	Long: `ローカルで編集したJIRAチケットとリモートのJIRAチケットの差分を表示します。
差分を計算する前に~/.cache/gojiraにリモートのチケットをfetchします。`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("ローカルとリモートのJIRAチケットの差分を表示します（ディレクトリ: %s, フォーマット: %s）\n", diffDir, diffFormat)

		// 2. キャッシュディレクトリを確保
		cacheDir, err := config.EnsureCacheDir()
		if err != nil {
			return fmt.Errorf("キャッシュディレクトリの作成に失敗しました: %v", err)
		}

		// 4. ローカルとキャッシュの差分を検出
		fmt.Printf("ローカルディレクトリ %s とキャッシュの差分を検出中...\n", diffDir)
		diffs, err := ticket.CompareDirs(diffDir, cacheDir)
		if err != nil {
			return fmt.Errorf("差分の検出に失敗しました: %v", err)
		}

		// 5. 差分を表示
		if diffFormat == "json" {
			return displayDiffsAsJSON(diffs)
		} else {
			return displayDiffsAsText(diffs)
		}
	},
}

// displayDiffsAsText はテキスト形式で差分を表示します
func displayDiffsAsText(diffs []ticket.DiffResult) error {
	changedCount := 0
	unchangedCount := 0

	var output strings.Builder
	output.WriteString("\n=== 差分結果 ===")

	for _, diff := range diffs {
		if diff.HasDiff {
			changedCount++
			output.WriteString(fmt.Sprintf("\n\n[変更あり] %s (%s)\n", diff.Key, diff.FilePath))
			if diff.DiffText != "" {
				output.WriteString("差分:\n")
				output.WriteString(diff.DiffText)
			}
			output.WriteString("\n---")
		} else {
			unchangedCount++
		}
	}

	if unchangedCount > 0 {
		output.WriteString(fmt.Sprintf("\n\n[変更なし] %d件のチケットには変更がありません\n", unchangedCount))
	}

	output.WriteString(fmt.Sprintf("\n概要: %d件変更, %d件変更なし\n", changedCount, unchangedCount))

	return displayWithPager(output.String())
}

// displayDiffsAsJSON はJSON形式で差分を表示します
func displayDiffsAsJSON(diffs []ticket.DiffResult) error {
	output := map[string]interface{}{
		"summary": map[string]int{
			"changed":   0,
			"unchanged": 0,
		},
		"diffs": diffs,
	}

	// 統計を計算
	for _, diff := range diffs {
		if diff.HasDiff {
			output["summary"].(map[string]int)["changed"]++
		} else {
			output["summary"].(map[string]int)["unchanged"]++
		}
	}

	jsonBytes, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return fmt.Errorf("JSON出力の生成に失敗しました: %v", err)
	}

	return displayWithPager(string(jsonBytes))
}

// displayWithPager は内容をページャーで表示します
func displayWithPager(content string) error {
	// 環境変数PAGERを確認、なければlessを使用
	pager := os.Getenv("PAGER")
	if pager == "" {
		pager = "less"
	}

	// ページャーコマンドを実行
	cmd := exec.Command(pager)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	stdin, err := cmd.StdinPipe()
	if err != nil {
		// ページャーが使えない場合は標準出力に直接出力
		fmt.Print(content)
		return nil
	}

	// ページャーを起動
	if err := cmd.Start(); err != nil {
		// ページャーが使えない場合は標準出力に直接出力
		fmt.Print(content)
		return nil
	}

	// コンテンツをページャーに送信
	go func() {
		defer stdin.Close()
		io.WriteString(stdin, content)
	}()

	// ページャーの終了を待つ
	return cmd.Wait()
}

func init() {
	rootCmd.AddCommand(diffCmd)

	// フラグの設定
	diffCmd.Flags().StringVarP(&diffDir, "dir", "d", "./tmp", "比較対象のローカルディレクトリ")
	diffCmd.Flags().StringVarP(&diffFormat, "format", "f", "text", "出力フォーマット (text|json)")
}
