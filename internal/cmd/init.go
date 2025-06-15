package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/ktr0731/go-fuzzyfinder"
	"github.com/qawatake/tkt/internal/config"
	"github.com/qawatake/tkt/internal/ui"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "インタラクティブに設定ファイルを作成します。",
	Long: `インタラクティブに設定ファイルを作成します。
JIRAサーバーのURL、ログインメール、プロジェクト、ボードを選択して
カレントディレクトリにtkt.ymlを作成します。`,
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

type JiraIssueType struct {
	ID               string `json:"id"`
	Description      string `json:"description"`
	Name             string `json:"name"`
	UntranslatedName string `json:"untranslatedName"`
	Subtask          bool   `json:"subtask"`
	Scope            *struct {
		Type    string `json:"type"`
		Project struct {
			ID string `json:"id"`
		} `json:"project"`
	} `json:"scope"`
}

func runInit() error {
	fmt.Println("🔧 tkt設定セットアップ")
	fmt.Println("=======================")

	// 1. JIRAサーバーURLを入力
	serverURL, err := ui.PromptForText("JIRAサーバーのURL (必須):", "https://your-domain.atlassian.net", true)
	if err != nil {
		return fmt.Errorf("JIRAサーバーURL入力がキャンセルされました: %v", err)
	}

	// 2. ログインメールを入力
	loginEmail, err := ui.PromptForText("ログインメールアドレス (必須):", "your-email@company.com", true)
	if err != nil {
		return fmt.Errorf("ログインメール入力がキャンセルされました: %v", err)
	}

	// 3. APIトークンの確認
	apiToken := os.Getenv("JIRA_API_TOKEN")
	if apiToken == "" {
		fmt.Println("\n⚠️  JIRA_API_TOKEN環境変数が設定されていません。")
		fmt.Println("   Atlassian API Token (https://id.atlassian.com/manage-profile/security/api-tokens) を取得して、")
		fmt.Println("   環境変数 JIRA_API_TOKEN に設定してください。")
		continueSetup, err := ui.PromptForConfirmation("続行しますか？")
		if err != nil {
			return fmt.Errorf("確認入力がキャンセルされました: %v", err)
		}
		if !continueSetup {
			return fmt.Errorf("セットアップを中止しました")
		}
		apiToken = "dummy_token" // 一時的なダミートークン
	}

	// 4. プロジェクト一覧を取得
	projects, err := ui.WithSpinnerValue("プロジェクト一覧を取得中...", func() ([]JiraProject, error) {
		return fetchProjects(serverURL, loginEmail, apiToken)
	})
	if err != nil {
		return fmt.Errorf("プロジェクト一覧の取得に失敗しました: %v", err)
	}

	if len(projects) == 0 {
		return fmt.Errorf("アクセス可能なプロジェクトが見つかりません")
	}

	// 5. プロジェクトを選択
	fmt.Println("\n📋 プロジェクトを選択してください (入力してフィルタリング可能):")
	projectIdx, err := fuzzyfinder.Find(
		projects,
		func(i int) string {
			return fmt.Sprintf("%s (%s)", projects[i].Name, projects[i].Key)
		},
		fuzzyfinder.WithPreviewWindow(func(i, w, h int) string {
			return fmt.Sprintf("プロジェクト: %s\nキー: %s\nID: %s",
				projects[i].Name, projects[i].Key, projects[i].ID)
		}),
	)
	if err != nil {
		return fmt.Errorf("プロジェクトの選択がキャンセルされました: %v", err)
	}
	selectedProject := &projects[projectIdx]

	// 6. ボード一覧を取得
	boards, err := ui.WithSpinnerValue(fmt.Sprintf("プロジェクト '%s' のボード一覧を取得中...", selectedProject.Name), func() ([]JiraBoard, error) {
		return fetchBoards(serverURL, loginEmail, apiToken, selectedProject.Key)
	})
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
		fmt.Println("\n📊 ボードを選択してください (入力してフィルタリング可能):")
		boardIdx, err := fuzzyfinder.Find(
			boards,
			func(i int) string {
				return fmt.Sprintf("%s (ID: %d)", boards[i].Name, boards[i].ID)
			},
			fuzzyfinder.WithPreviewWindow(func(i, w, h int) string {
				return fmt.Sprintf("ボード: %s\nID: %d\nタイプ: %s",
					boards[i].Name, boards[i].ID, boards[i].Type)
			}),
		)
		if err != nil {
			return fmt.Errorf("ボードの選択がキャンセルされました: %v", err)
		}
		selectedBoard = &boards[boardIdx]
	}

	// 8. JQLを入力
	fmt.Println()
	defaultJQL := fmt.Sprintf("project = %s", selectedProject.Key)
	jqlInput, err := ui.PromptForText(fmt.Sprintf("JQL (デフォルト: %s):", defaultJQL), defaultJQL, false)
	if err != nil {
		return fmt.Errorf("JQL入力がキャンセルされました: %v", err)
	}
	if jqlInput == "" {
		jqlInput = defaultJQL
	}

	// 9. Issue typesを取得
	issueTypes, err := ui.WithSpinnerValue("Issue Types一覧を取得中...", func() ([]JiraIssueType, error) {
		return fetchIssueTypes(serverURL, loginEmail, apiToken)
	})
	if err != nil {
		return fmt.Errorf("issue Types一覧の取得に失敗しました: %v", err)
	}

	// 10. ディレクトリを入力
	defaultDirectory := "tmp"
	directoryInput, err := ui.PromptForText(fmt.Sprintf("マークダウンファイル格納ディレクトリ (デフォルト: %s):", defaultDirectory), defaultDirectory, false)
	if err != nil {
		return fmt.Errorf("ディレクトリ入力がキャンセルされました: %v", err)
	}
	if directoryInput == "" {
		directoryInput = defaultDirectory
	}

	// 11. 設定ファイルを作成
	cfg := &config.Config{
		AuthType:  "basic",
		Login:     loginEmail,
		Server:    serverURL,
		JQL:       jqlInput,
		Timezone:  "Asia/Tokyo",
		Directory: directoryInput,
	}

	// Project情報を設定
	cfg.Project.Key = selectedProject.Key
	cfg.Project.ID = selectedProject.ID
	cfg.Project.Type = "software"

	// Board情報を設定
	cfg.Board.ID = selectedBoard.ID
	cfg.Board.Name = selectedBoard.Name
	cfg.Board.Type = selectedBoard.Type

	// Issue Types情報を設定
	for _, issueType := range issueTypes {
		issueTypeConfig := config.IssueType{
			ID:               issueType.ID,
			Description:      issueType.Description,
			Name:             issueType.Name,
			UntranslatedName: issueType.UntranslatedName,
			Subtask:          issueType.Subtask,
		}

		// Scopeがnullでない場合のみ設定
		if issueType.Scope != nil && (issueType.Scope.Type != "" || issueType.Scope.Project.ID != "") {
			issueTypeConfig.Scope = &config.IssueTypeScope{
				Type: issueType.Scope.Type,
				Project: config.IssueTypeScopeProject{
					ID: issueType.Scope.Project.ID,
				},
			}
		}

		cfg.Issue.Types = append(cfg.Issue.Types, issueTypeConfig)
	}

	// 12. 設定ファイルを保存 (tkt.ymlをカレントディレクトリに作成)
	configFile := "tkt.yml"
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("設定ファイルのマーシャルに失敗しました: %v", err)
	}

	if err := os.WriteFile(configFile, data, 0644); err != nil {
		return fmt.Errorf("設定ファイルの書き込みに失敗しました: %v", err)
	}

	fmt.Println("\n✅ 設定が完了しました！")
	fmt.Printf("   設定ファイル: %s (カレントディレクトリ)\n", configFile)
	fmt.Printf("   プロジェクト: %s (%s)\n", selectedProject.Name, selectedProject.Key)
	fmt.Printf("   ボード: %s (ID: %d)\n", selectedBoard.Name, selectedBoard.ID)

	return nil
}

func fetchProjects(serverURL, email, apiToken string) ([]JiraProject, error) {
	// 直近20件だ十分なはず。
	url := serverURL + "/rest/api/3/project?recent=20"

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

func fetchIssueTypes(serverURL, email, apiToken string) ([]JiraIssueType, error) {
	url := serverURL + "/rest/api/3/issuetype"

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

	var issueTypes []JiraIssueType
	if err := json.NewDecoder(resp.Body).Decode(&issueTypes); err != nil {
		return nil, err
	}

	return issueTypes, nil
}
