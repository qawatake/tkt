package cmd

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/gojira/gojira/internal/config"
	"github.com/gojira/gojira/internal/verbose"
	"github.com/gojira/gojira/pkg/markdown"
	"github.com/spf13/cobra"
)

var (
	queryDir string
)

var queryCmd = &cobra.Command{
	Use:     "query",
	Aliases: []string{"q"},
	Short:   "ローカルファイルのフロントマターをDuckDBで検索",
	Long: `ローカルのマークダウンファイルのフロントマターをJSONに変換し、
DuckDBのREPLを起動してSQLクエリを実行できます。
REPLを終了すると一時ファイルは自動的に削除されます。`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// 1. 設定ファイルを読み込む
		cfg, err := config.LoadConfig()
		if err != nil {
			return fmt.Errorf("設定ファイルの読み込みに失敗しました: %v", err)
		}

		// queryDirが指定されていない場合は設定ファイルのディレクトリを使用
		if queryDir == "" {
			if cfg.Directory == "" {
				return fmt.Errorf("設定ファイルにdirectoryが設定されていません。gojira initで設定してください")
			}
			queryDir = cfg.Directory
		}

		// 2. マークダウンファイルを検索
		var markdownFiles []string
		err = filepath.WalkDir(queryDir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !d.IsDir() && strings.HasSuffix(path, ".md") {
				markdownFiles = append(markdownFiles, path)
			}
			return nil
		})
		if err != nil {
			return fmt.Errorf("ファイル検索に失敗しました: %v", err)
		}

		if len(markdownFiles) == 0 {
			return fmt.Errorf("マークダウンファイルが見つかりません")
		}

		verbose.Printf("%d 件のマークダウンファイルを発見しました\n", len(markdownFiles))

		// 3. フロントマターを抽出してJSONに変換
		var allFrontmatters []map[string]interface{}
		for _, file := range markdownFiles {
			content, err := os.ReadFile(file)
			if err != nil {
				verbose.Printf("警告: %s の読み込みに失敗しました: %v\n", file, err)
				continue
			}

			frontmatter, _, err := markdown.ParseFrontMatter(string(content))
			if err != nil {
				verbose.Printf("警告: %s のフロントマターパースに失敗しました: %v\n", file, err)
				continue
			}

			if frontmatter != nil {
				// ファイルパスも追加
				frontmatter["_file_path"] = file
				allFrontmatters = append(allFrontmatters, frontmatter)
			}
		}

		if len(allFrontmatters) == 0 {
			return fmt.Errorf("有効なフロントマターが見つかりません")
		}

		verbose.Printf("%d 件のフロントマターを抽出しました\n", len(allFrontmatters))

		// 4. 一時JSONファイルを作成
		tempFile := filepath.Join("/tmp", fmt.Sprintf("gojira_query_%d.json", time.Now().Unix()))
		jsonData, err := json.MarshalIndent(allFrontmatters, "", "  ")
		if err != nil {
			return fmt.Errorf("JSON変換に失敗しました: %v", err)
		}

		err = os.WriteFile(tempFile, jsonData, 0644)
		if err != nil {
			return fmt.Errorf("一時ファイルの作成に失敗しました: %v", err)
		}

		verbose.Printf("一時ファイルを作成しました: %s\n", tempFile)

		// 5. DuckDBのREPLを起動
		verbose.Println("DuckDBのREPLを起動中...")
		verbose.Printf("データベースのテーブル名: tickets\n")
		verbose.Printf("使用例: SELECT * FROM tickets WHERE status = 'To Do';\n")
		verbose.Println("終了するには .exit を入力してください")

		// 初期化SQLファイルを作成
		initSQL := fmt.Sprintf("CREATE TABLE tickets AS SELECT * FROM read_json_auto('%s');", tempFile)
		initFile := filepath.Join("/tmp", fmt.Sprintf("gojira_init_%d.sql", time.Now().Unix()))
		err = os.WriteFile(initFile, []byte(initSQL), 0644)
		if err != nil {
			os.Remove(tempFile)
			return fmt.Errorf("初期化SQLファイルの作成に失敗しました: %v", err)
		}

		// DuckDBコマンドを構築（初期化SQLファイルを読み込んでREPLを起動）
		duckdbCmd := exec.Command("duckdb", ":memory:", "-init", initFile)
		duckdbCmd.Stdin = os.Stdin
		duckdbCmd.Stdout = os.Stdout
		duckdbCmd.Stderr = os.Stderr

		// DuckDBを実行
		err = duckdbCmd.Run()

		// 初期化ファイルも削除
		os.Remove(initFile)

		// 6. 一時ファイルを削除
		os.Remove(tempFile)
		verbose.Printf("\n一時ファイルを削除しました: %s\n", tempFile)

		// DuckDBの正常終了（ユーザーが.exitで終了）は成功として扱う
		if err != nil {
			if exitError, ok := err.(*exec.ExitError); ok {
				// 終了コード0以外でも、ユーザーが意図的に終了した場合は成功とする
				verbose.Printf("DuckDBが終了しました (exit code: %d)\n", exitError.ExitCode())
			} else {
				return fmt.Errorf("DuckDBの実行に失敗しました: %v", err)
			}
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(queryCmd)

	// フラグの設定
	queryCmd.Flags().StringVarP(&queryDir, "dir", "d", "", "検索対象ディレクトリ")
}
