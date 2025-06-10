package cmd

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/ktr0731/go-fuzzyfinder"
	"github.com/qawatake/tkt/internal/config"
	"github.com/qawatake/tkt/internal/jira"
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

	// JIRAクライアントを作成
	client, err := jira.NewClient(cfg)
	if err != nil {
		return fmt.Errorf("JIRAクライアントの作成に失敗しました: %v", err)
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

	// 2. チケットタイプを選択
	ticketTypes := []string{"Story", "Bug", "Task", "Epic", "Subtask"}
	fmt.Println("\n📋 チケットタイプを選択してください:")

	typeIdx, err := fuzzyfinder.Find(
		ticketTypes,
		func(i int) string {
			return ticketTypes[i]
		},
		fuzzyfinder.WithPreviewWindow(func(i, w, h int) string {
			descriptions := map[string]string{
				"Story":   "新機能や改善要求",
				"Bug":     "不具合の修正",
				"Task":    "作業タスク",
				"Epic":    "大きな機能の集合体",
				"Subtask": "他のチケットのサブタスク",
			}
			return fmt.Sprintf("タイプ: %s\n説明: %s", ticketTypes[i], descriptions[ticketTypes[i]])
		}),
	)
	if err != nil {
		return fmt.Errorf("チケットタイプの選択がキャンセルされました: %v", err)
	}
	selectedType := strings.ToLower(ticketTypes[typeIdx])

	// 3. ボディをvimエディタで入力
	fmt.Println("\n📝 ボディを編集します (vimエディタが開きます)...")
	body, err := openEditor()
	if err != nil {
		return fmt.Errorf("エディタの起動に失敗しました: %v", err)
	}

	// 4. チケットを作成
	newTicket := &ticket.Ticket{
		Title:     title,
		Type:      selectedType,
		Body:      body,
		Status:    "To Do",
		Reporter:  cfg.Login,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	fmt.Println("\n🚀 JIRAチケットを作成中...")
	createdTicket, err := ui.WithSpinnerValue("チケットを作成中...", func() (*ticket.Ticket, error) {
		return client.CreateIssue(newTicket)
	})
	if err != nil {
		return fmt.Errorf("チケットの作成に失敗しました: %v", err)
	}

	// 5. ローカルファイルとして保存
	filePath, err := ui.WithSpinnerValue("ローカルファイルを保存中...", func() (string, error) {
		return createdTicket.SaveToFile(cfg.Directory)
	})
	if err != nil {
		return fmt.Errorf("ローカルファイルの保存に失敗しました: %v", err)
	}

	fmt.Println("\n✅ チケットが作成されました！")
	fmt.Printf("   チケットキー: %s\n", createdTicket.Key)
	fmt.Printf("   タイトル: %s\n", createdTicket.Title)
	fmt.Printf("   タイプ: %s\n", createdTicket.Type)
	fmt.Printf("   ファイル: %s\n", filePath)

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
