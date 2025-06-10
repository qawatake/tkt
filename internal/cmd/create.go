package cmd

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/ktr0731/go-fuzzyfinder"
	"github.com/qawatake/tkt/internal/config"
	"github.com/qawatake/tkt/internal/ticket"
	"github.com/qawatake/tkt/internal/ui"
	"github.com/spf13/cobra"
)

var createCmd = &cobra.Command{
	Use:   "create",
	Short: "新しいJIRAチケットをインタラクティブに作成します",
	Long: `新しいJIRAチケットをインタラクティブに作成します。
タイトル、タイプを入力し、vimエディタでボディを編集できます。`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runCreate()
	},
}

func init() {
	rootCmd.AddCommand(createCmd)
}

func runCreate() error {
	// 設定ファイルを読み込み
	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("設定ファイルの読み込みに失敗しました: %v\n'tkt init' コマンドで設定ファイルを作成してください", err)
	}

	scanner := bufio.NewScanner(os.Stdin)

	fmt.Println("🎫 新しいJIRAチケット作成")
	fmt.Println("========================")

	// 1. タイトルを入力
	fmt.Print("チケットタイトル (必須): ")
	if !scanner.Scan() {
		return fmt.Errorf("入力エラー")
	}
	title := strings.TrimSpace(scanner.Text())
	if title == "" {
		return fmt.Errorf("タイトルは必須です")
	}

	// 2. チケットタイプを選択 (プロジェクトに対応するもののみ)
	var availableTypes []config.IssueType

	// 現在のプロジェクトのIssue Typesのみをフィルタリング
	for _, issueType := range cfg.Issue.Types {
		// Scopeがない（グローバル）またはプロジェクトIDが一致する場合
		if issueType.Scope == nil || issueType.Scope.Project.ID == "" || issueType.Scope.Project.ID == cfg.Project.ID {
			availableTypes = append(availableTypes, issueType)
		}
	}

	if len(availableTypes) == 0 {
		return fmt.Errorf("プロジェクト '%s' に対応するチケットタイプが見つかりません", cfg.Project.Key)
	}

	fmt.Println("\n📋 チケットタイプを選択してください:")

	typeIdx, err := fuzzyfinder.Find(
		availableTypes,
		func(i int) string {
			return availableTypes[i].Name
		},
		fuzzyfinder.WithPreviewWindow(func(i, w, h int) string {
			t := availableTypes[i]
			return fmt.Sprintf("タイプ: %s\nID: %s\nサブタスク: %t", t.Name, t.ID, t.Subtask)
		}),
	)
	if err != nil {
		return fmt.Errorf("チケットタイプの選択がキャンセルされました: %v", err)
	}
	selectedType := availableTypes[typeIdx].Name

	// 3. ボディをvimエディタで入力
	fmt.Println("\n📝 ボディを編集します (vimエディタが開きます)...")
	body, err := openEditor()
	if err != nil {
		return fmt.Errorf("エディタの起動に失敗しました: %v", err)
	}

	// 4. ローカルチケットを作成 (keyは空文字列、リモートが採番)
	newTicket := &ticket.Ticket{
		Key:   "", // リモートが採番するため空文字列
		Title: title,
		Type:  selectedType,
		Body:  body,
	}

	// 5. ローカルファイルとして保存
	fmt.Println("\n💾 ローカルファイルを保存中...")
	filePath, err := ui.WithSpinnerValue("ローカルファイルを保存中...", func() (string, error) {
		return newTicket.SaveToFile(cfg.Directory)
	})
	if err != nil {
		return fmt.Errorf("ローカルファイルの保存に失敗しました: %v", err)
	}

	fmt.Println("\n✅ ローカルチケットが作成されました！")
	fmt.Printf("   タイトル: %s\n", newTicket.Title)
	fmt.Printf("   タイプ: %s\n", newTicket.Type)
	fmt.Printf("   ファイル: %s\n", filePath)
	fmt.Printf("   次のステップ: 'tkt push' でJIRAに同期してキーを取得\n")

	return nil
}

// openEditor はvimエディタを開いてユーザーに入力させます
func openEditor() (string, error) {
	// 一時ファイルを作成
	tmpFile, err := os.CreateTemp("", "tkt-create-*.md")
	if err != nil {
		return "", fmt.Errorf("一時ファイルの作成に失敗しました: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	tmpFile.Close()

	// vimエディタを起動 (insertモードで開始)
	cmd := exec.Command("vim", "+startinsert", tmpFile.Name())
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("vimエディタの実行に失敗しました: %v", err)
	}

	// ファイルの内容を読み取り
	content, err := os.ReadFile(tmpFile.Name())
	if err != nil {
		return "", fmt.Errorf("ファイルの読み取りに失敗しました: %v", err)
	}

	body := strings.TrimSpace(string(content))
	return body, nil
}
