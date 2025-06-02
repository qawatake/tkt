package jira

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	jiralib "github.com/andygrunwald/go-jira"
	"github.com/ankitpokhrel/jira-cli/pkg/adf"
	"github.com/ankitpokhrel/jira-cli/pkg/jira"
	"github.com/ankitpokhrel/jira-cli/pkg/md"
	"github.com/gojira/gojira/internal/config"
	"github.com/gojira/gojira/internal/ticket"
)

// Client はJIRA APIクライアントのラッパーです
type Client struct {
	jiraClient    *jiralib.Client
	jiraCLIClient *jira.Client
	config        *config.Config
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

	jiraCLIClient := jira.NewClient(newJIRACLIConfig(cfg))

	return &Client{
		jiraClient:    jiraClient,
		jiraCLIClient: jiraCLIClient,
		config:        cfg,
	}, nil
}

func newJIRACLIConfig(cfg *config.Config) jira.Config {
	return jira.Config{
		Server:   cfg.Server,
		Login:    cfg.Login,
		APIToken: getAPIToken(),
	}
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
func (c *Client) FetchIssues() ([]ticket.Ticket, error) {
	// まずプロジェクトが存在するか確認
	if err := c.validateProject(); err != nil {
		return nil, err
	}

	// JQLクエリを作成
	jql := c.config.JQL
	if jql == "" {
		jql = fmt.Sprintf("project = %s", c.config.Project.Key)
	}

	const limitRequestCount = 100 // 安全のための上限
	const maxResults = 100        // これが上限っぽい。(500にしても100でcapされた。)
	issues := make([]*jira.Issue, 0, 1000)
	var offset uint = 0
	for range limitRequestCount {
		fmt.Printf("🎉 offset=%d\n", offset)
		result, err := c.jiraCLIClient.Search(jql, offset, maxResults)
		if err != nil {
			return nil, err
		}
		issues = append(issues, result.Issues...)
		offset += uint(len(result.Issues))
		if offset >= uint(result.Total) {
			break
		}
	}

	fmt.Printf("JQLクエリ: %s\n", jql)

	tickets := make([]ticket.Ticket, 0, len(issues))
	for _, issue := range issues {
		ticket := convertJiraCLIIssueToTicket(issue)
		tickets = append(tickets, ticket)
	}

	return tickets, nil
}

func convertJiraCLIIssueToTicket(issue *jira.Issue) ticket.Ticket {
	ticket := ticket.Ticket{
		Key:    issue.Key,
		Title:  issue.Fields.Summary,
		Type:   strings.ToLower(issue.Fields.IssueType.Name),
		Status: issue.Fields.Status.Name,
	}

	adfBody := ifaceToADF(issue.Fields.Description)
	ticket.Body = adf.NewTranslator(adfBody, adf.NewJiraMarkdownTranslator()).Translate()

	if issue.Fields.Parent != nil {
		ticket.ParentKey = issue.Fields.Parent.Key
	}

	if issue.Fields.Assignee.Name != "" {
		ticket.Assignee = issue.Fields.Assignee.Name
	}

	if issue.Fields.Reporter.Name != "" {
		ticket.Reporter = issue.Fields.Reporter.Name
	}

	// Parse timestamps
	if createdTime, err := time.Parse(time.RFC3339, issue.Fields.Created); err == nil {
		ticket.CreatedAt = createdTime
	}

	if updatedTime, err := time.Parse(time.RFC3339, issue.Fields.Updated); err == nil {
		ticket.UpdatedAt = updatedTime
	}

	return ticket
}

func ifaceToADF(v interface{}) *adf.ADF {
	if v == nil {
		return nil
	}

	var doc *adf.ADF

	js, err := json.Marshal(v)
	if err != nil {
		return nil // ignore invalid data
	}
	if err = json.Unmarshal(js, &doc); err != nil {
		return nil // ignore invalid data
	}

	return doc
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
func (c *Client) UpdateIssue(ticket ticket.Ticket) error {
	err := c.jiraCLIClient.Edit(ticket.Key, &jira.EditRequest{
		IssueType:      ticket.Type,
		Summary:        ticket.Title,
		ParentIssueKey: ticket.ParentKey,
		Body:           md.ToJiraMD(ticket.Body),
	})
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
