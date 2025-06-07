package cmd

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/gojira/gojira/internal/config"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "インタラクティブに設定ファイルを作成",
	Long: `インタラクティブに設定ファイルを作成します。
JIRAサーバーのURL、ログインメール、プロジェクト、ボードを選択して
カレントディレクトリにticket.ymlを作成します。`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runInit()
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
}

type JiraProject struct {
	Key  string `json:"key"`
	Name string `json:"name"`
	ID   string `json:"id"`
}

type JiraBoard struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
}

func runInit() error {
	scanner := bufio.NewScanner(os.Stdin)

	fmt.Println("🔧 Gojira設定セットアップ")
	fmt.Println("=======================")
	fmt.Println()

	// 1. JIRAサーバーURLを入力
	fmt.Print("JIRAサーバーのURL (例: https://your-domain.atlassian.net): ")
	if !scanner.Scan() {
		return fmt.Errorf("入力エラー")
	}
	serverURL := strings.TrimSpace(scanner.Text())
	if serverURL == "" {
		return fmt.Errorf("JIRAサーバーURLは必須です")
	}

	// 2. ログインメールを入力
	fmt.Print("ログインメールアドレス: ")
	if !scanner.Scan() {
		return fmt.Errorf("入力エラー")
	}
	loginEmail := strings.TrimSpace(scanner.Text())
	if loginEmail == "" {
		return fmt.Errorf("ログインメールアドレスは必須です")
	}

	// 3. APIトークンの確認
	apiToken := os.Getenv("JIRA_API_TOKEN")
	if apiToken == "" {
		fmt.Println()
		fmt.Println("⚠️  JIRA_API_TOKEN環境変数が設定されていません。")
		fmt.Println("   Atlassian API Token (https://id.atlassian.com/manage-profile/security/api-tokens) を取得して、")
		fmt.Println("   環境変数 JIRA_API_TOKEN に設定してください。")
		fmt.Println()
		fmt.Print("続行しますか？ (y/N): ")
		if !scanner.Scan() {
			return fmt.Errorf("入力エラー")
		}
		if strings.ToLower(strings.TrimSpace(scanner.Text())) != "y" {
			return fmt.Errorf("セットアップを中止しました")
		}
		apiToken = "dummy_token" // 一時的なダミートークン
	}

	// 4. プロジェクト一覧を取得
	fmt.Println()
	fmt.Println("📋 プロジェクト一覧を取得中...")

	projects, err := fetchProjects(serverURL, loginEmail, apiToken)
	if err != nil {
		return fmt.Errorf("プロジェクト一覧の取得に失敗しました: %v", err)
	}

	if len(projects) == 0 {
		return fmt.Errorf("アクセス可能なプロジェクトが見つかりません")
	}

	// 5. プロジェクトを選択
	fmt.Println()
	fmt.Println("📋 利用可能なプロジェクト:")
	for i, project := range projects {
		fmt.Printf("  %d) %s (%s)\n", i+1, project.Name, project.Key)
	}

	var selectedProject *JiraProject
	for {
		fmt.Print("プロジェクトを選択してください: ")
		if !scanner.Scan() {
			return fmt.Errorf("入力エラー")
		}

		choice, err := strconv.Atoi(strings.TrimSpace(scanner.Text()))
		if err != nil || choice < 1 || choice > len(projects) {
			fmt.Println("無効な選択です。再入力してください。")
			continue
		}

		selectedProject = &projects[choice-1]
		break
	}

	// 6. ボード一覧を取得
	fmt.Println()
	fmt.Printf("📊 プロジェクト '%s' のボード一覧を取得中...\n", selectedProject.Name)

	boards, err := fetchBoards(serverURL, loginEmail, apiToken, selectedProject.Key)
	if err != nil {
		return fmt.Errorf("ボード一覧の取得に失敗しました: %v", err)
	}

	var selectedBoard *JiraBoard
	if len(boards) == 0 {
		fmt.Println("⚠️  利用可能なボードが見つかりませんでした。デフォルト設定を使用します。")
		selectedBoard = &JiraBoard{
			ID:   0,
			Name: "Default",
			Type: "scrum",
		}
	} else {
		// 7. ボードを選択
		fmt.Println()
		fmt.Println("📊 利用可能なボード:")
		for i, board := range boards {
			fmt.Printf("  %d) %s (ID: %d, Type: %s)\n", i+1, board.Name, board.ID, board.Type)
		}

		for {
			fmt.Print("ボードを選択してください: ")
			if !scanner.Scan() {
				return fmt.Errorf("入力エラー")
			}

			choice, err := strconv.Atoi(strings.TrimSpace(scanner.Text()))
			if err != nil || choice < 1 || choice > len(boards) {
				fmt.Println("無効な選択です。再入力してください。")
				continue
			}

			selectedBoard = &boards[choice-1]
			break
		}
	}

	// 8. JQLを入力
	fmt.Println()
	defaultJQL := fmt.Sprintf("project = %s", selectedProject.Key)
	fmt.Printf("JQL (デフォルト: %s): ", defaultJQL)
	if !scanner.Scan() {
		return fmt.Errorf("入力エラー")
	}

	jqlInput := strings.TrimSpace(scanner.Text())
	if jqlInput == "" {
		jqlInput = defaultJQL
	}

	// 9. 設定ファイルを作成
	cfg := &config.Config{
		AuthType: "basic",
		Login:    loginEmail,
		Server:   serverURL,
		JQL:      jqlInput,
		Timezone: "Asia/Tokyo",
	}

	// Project情報を設定
	cfg.Project.Key = selectedProject.Key
	cfg.Project.Type = "software"

	// Board情報を設定
	cfg.Board.ID = selectedBoard.ID
	cfg.Board.Name = selectedBoard.Name
	cfg.Board.Type = selectedBoard.Type

	// 9. 設定ファイルを保存 (ticket.ymlをカレントディレクトリに作成)
	configFile := "ticket.yml"
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("設定ファイルのマーシャルに失敗しました: %v", err)
	}

	if err := os.WriteFile(configFile, data, 0644); err != nil {
		return fmt.Errorf("設定ファイルの書き込みに失敗しました: %v", err)
	}

	fmt.Println()
	fmt.Println("✅ 設定が完了しました！")
	fmt.Printf("   設定ファイル: %s (カレントディレクトリ)\n", configFile)
	fmt.Printf("   プロジェクト: %s (%s)\n", selectedProject.Name, selectedProject.Key)
	fmt.Printf("   ボード: %s (ID: %d)\n", selectedBoard.Name, selectedBoard.ID)
	fmt.Println()
	fmt.Println("💡 使用方法:")
	fmt.Println("   gojira fetch  # チケットを取得")
	fmt.Println("   gojira push   # チケットを更新")
	fmt.Println()

	return nil
}

func fetchProjects(serverURL, email, apiToken string) ([]JiraProject, error) {
	url := serverURL + "/rest/api/3/project"

	req, err := http.NewRequestWithContext(context.Background(), "GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.SetBasicAuth(email, apiToken)
	req.Header.Set("Accept", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("JIRA API request failed: %s", resp.Status)
	}

	var projects []JiraProject
	if err := json.NewDecoder(resp.Body).Decode(&projects); err != nil {
		return nil, err
	}

	return projects, nil
}

func fetchBoards(serverURL, email, apiToken, projectKey string) ([]JiraBoard, error) {
	url := serverURL + "/rest/agile/1.0/board?projectKeyOrId=" + projectKey

	req, err := http.NewRequestWithContext(context.Background(), "GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.SetBasicAuth(email, apiToken)
	req.Header.Set("Accept", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("JIRA API request failed: %s", resp.Status)
	}

	var response struct {
		Values []JiraBoard `json:"values"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, err
	}

	return response.Values, nil
}
