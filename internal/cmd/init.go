package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/charmbracelet/huh"
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
}

func runInit() error {
	fmt.Println("🔧 tkt設定セットアップ")
	fmt.Println("=======================")

	var serverURL, loginEmail string
	var continueSetup bool

	// 1. 基本設定フォーム
	basicForm := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("JIRAサーバーのURL").
				Description("JIRAインスタンスのベースURL (例: https://your-domain.atlassian.net)").
				Value(&serverURL).
				Validate(func(s string) error {
					if s == "" {
						return fmt.Errorf("JIRAサーバーURLは必須です")
					}
					return nil
				}),

			huh.NewInput().
				Title("ログインメールアドレス").
				Description("JIRA認証に使用するメールアドレス (例: your-email@company.com)").
				Value(&loginEmail).
				Validate(func(s string) error {
					if s == "" {
						return fmt.Errorf("ログインメールは必須です")
					}
					return nil
				}),
		),
	).WithTheme(huh.ThemeBase())

	err := basicForm.Run()
	if err != nil {
		return fmt.Errorf("基本設定の入力がキャンセルされました: %v", err)
	}

	// 2. APIトークンの確認
	apiToken := os.Getenv("JIRA_API_TOKEN")
	if apiToken == "" {
		fmt.Println("\n⚠️  JIRA_API_TOKEN環境変数が設定されていません。")
		fmt.Println("   Atlassian API Token (https://id.atlassian.com/manage-profile/security/api-tokens) を取得して、")
		fmt.Println("   環境変数 JIRA_API_TOKEN に設定してください。")

		confirmForm := huh.NewForm(
			huh.NewGroup(
				huh.NewConfirm().
					Title("続行しますか？").
					Description("APIトークンなしでもセットアップを続行できますが、後で設定が必要です").
					Value(&continueSetup),
			),
		).WithTheme(huh.ThemeBase())

		err := confirmForm.Run()
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

	// 8. JQLとディレクトリ設定フォーム
	var jqlInput, directoryInput string

	fmt.Println()
	updatedAtThreshold := time.Now().AddDate(0, -6, 0)
	defaultJQL := fmt.Sprintf("project = %s AND updated >= '%s'", selectedProject.Key, updatedAtThreshold.Format("2006-01-02"))
	defaultDirectory := "tmp"

	jqlInput = defaultJQL
	directoryInput = defaultDirectory

	settingsForm := huh.NewForm(
		huh.NewGroup(
			huh.NewText().
				Title("JQL (JIRA Query Language)").
				Description(fmt.Sprintf("チケット検索条件を指定 (デフォルト: %s)", defaultJQL)).
				Value(&jqlInput),

			huh.NewInput().
				Title("マークダウンファイル格納ディレクトリ").
				Description(fmt.Sprintf("ローカルに保存するチケットファイルの場所 (デフォルト: %s)", defaultDirectory)).
				Value(&directoryInput),
		),
	).WithTheme(huh.ThemeBase())

	err = settingsForm.Run()
	if err != nil {
		return fmt.Errorf("設定入力がキャンセルされました: %v", err)
	}

	if jqlInput == "" {
		jqlInput = defaultJQL
	}
	if directoryInput == "" {
		directoryInput = defaultDirectory
	}

	// 9. Issue typesを取得
	issueTypes, err := ui.WithSpinnerValue("Issue Types一覧を取得中...", func() ([]JiraIssueType, error) {
		return fetchIssueTypes(serverURL, loginEmail, apiToken, selectedProject.ID)
	})
	if err != nil {
		return fmt.Errorf("issue Types一覧の取得に失敗しました: %v", err)
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

func fetchIssueTypes(serverURL, email, apiToken, projectID string) ([]JiraIssueType, error) {
	v := url.Values{}
	v.Add("projectId", projectID)
	url := serverURL + "/rest/api/3/issuetype/project?" + v.Encode()

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
