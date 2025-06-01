package jira

import (
	"fmt"
	"os"

	jiralib "github.com/andygrunwald/go-jira"
	"github.com/gojira/gojira/internal/config"
)

// Client はJIRA APIクライアントのラッパーです
type Client struct {
	jiraClient *jiralib.Client
	config     *config.Config
}

// NewClient は新しいJIRA APIクライアントを作成します
func NewClient(cfg *config.Config) (*Client, error) {
	var jiraClient *jiralib.Client
	var err error
	
	// 認証タイプに応じたクライアントを作成
	switch cfg.AuthType {
	case "basic":
		// 環境変数からAPIトークンを取得
		apiToken := getAPIToken()
		if apiToken == "" {
			return nil, fmt.Errorf("JIRA_API_TOKEN環境変数が設定されていません")
		}
		
		tp := jiralib.BasicAuthTransport{
			Username: cfg.Login,
			Password: apiToken,
		}
		jiraClient, err = jiralib.NewClient(tp.Client(), cfg.Server)
		
	case "bearer":
		// 環境変数からAPIトークンを取得
		apiToken := getAPIToken()
		if apiToken == "" {
			return nil, fmt.Errorf("JIRA_API_TOKEN環境変数が設定されていません")
		}
		
		tp := jiralib.BearerAuthTransport{
			Token: apiToken,
		}
		jiraClient, err = jiralib.NewClient(tp.Client(), cfg.Server)
		
	default:
		return nil, fmt.Errorf("サポートされていない認証タイプです: %s", cfg.AuthType)
	}
	
	if err != nil {
		return nil, fmt.Errorf("JIRAクライアントの作成に失敗しました: %v", err)
	}
	
	return &Client{
		jiraClient: jiraClient,
		config:     cfg,
	}, nil
}

// getAPIToken は環境変数からAPIトークンを取得します
func getAPIToken() string {
	token := os.Getenv("JIRA_API_TOKEN")
	if token == "" {
		// 開発用のダミートークン（実際の環境では設定してください）
		return "dummy_token"
	}
	return token
}

// FetchIssues はJQLに基づいてJIRAチケットを取得します
func (c *Client) FetchIssues() ([]jiralib.Issue, error) {
	// まずプロジェクトが存在するか確認
	if err := c.validateProject(); err != nil {
		return nil, err
	}

	// JQLクエリを作成
	jql := c.config.JQL
	if jql == "" {
		jql = fmt.Sprintf("project = %s", c.config.Project.Key)
	}
	
	fmt.Printf("JQLクエリ: %s\n", jql)
	
	// JIRAからチケットを取得
	issues, _, err := c.jiraClient.Issue.Search(jql, &jiralib.SearchOptions{
		MaxResults: 1000, // 最大結果数
		Fields:     []string{"summary", "description", "issuetype", "status", "assignee", "reporter", "created", "updated", "parent"},
	})
	
	if err != nil {
		return nil, fmt.Errorf("JIRAチケットの取得に失敗しました: %v", err)
	}
	
	return issues, nil
}

// validateProject はプロジェクトが存在するか確認します
func (c *Client) validateProject() error {
	project, _, err := c.jiraClient.Project.Get(c.config.Project.Key)
	if err != nil {
		return fmt.Errorf("プロジェクト '%s' が見つかりません。設定ファイルのproject.keyを確認してください: %v", c.config.Project.Key, err)
	}
	
	fmt.Printf("プロジェクト確認: %s (%s)\n", project.Name, project.Key)
	return nil
}

// UpdateIssue はJIRAチケットを更新します
func (c *Client) UpdateIssue(issue *jiralib.Issue) error {
	_, _, err := c.jiraClient.Issue.Update(issue)
	if err != nil {
		return fmt.Errorf("JIRAチケットの更新に失敗しました: %v", err)
	}
	
	return nil
}

// CreateIssue は新しいJIRAチケットを作成します
func (c *Client) CreateIssue(issueType, summary, description, parentKey string) (*jiralib.Issue, error) {
	// チケットタイプIDを取得
	typeID := ""
	for _, t := range c.config.Issue.Types {
		if t.Handle == issueType {
			typeID = t.ID
			break
		}
	}
	
	if typeID == "" {
		return nil, fmt.Errorf("チケットタイプが見つかりません: %s", issueType)
	}
	
	// チケット作成用のフィールドを準備
	fields := jiralib.IssueFields{
		Project: jiralib.Project{
			Key: c.config.Project.Key,
		},
		Type: jiralib.IssueType{
			ID: typeID,
		},
		Summary:     summary,
		Description: description,
	}
	
	// 親チケットがある場合は設定
	if parentKey != "" {
		fields.Parent = &jiralib.Parent{
			Key: parentKey,
		}
	}
	
	// チケットを作成
	issue := jiralib.Issue{
		Fields: &fields,
	}
	
	newIssue, _, err := c.jiraClient.Issue.Create(&issue)
	if err != nil {
		return nil, fmt.Errorf("JIRAチケットの作成に失敗しました: %v", err)
	}
	
	return newIssue, nil
}
