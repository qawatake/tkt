package jira

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"slices"
	"strings"
	"time"

	jiralib "github.com/andygrunwald/go-jira"
	"github.com/ankitpokhrel/jira-cli/pkg/jira"
	"github.com/ankitpokhrel/jira-cli/pkg/md"
	"github.com/gojira/gojira/internal/adf"
	"github.com/gojira/gojira/internal/config"
	"github.com/gojira/gojira/internal/derrors"
	"github.com/gojira/gojira/internal/ticket"
	"github.com/k1LoW/errors"
	"github.com/sourcegraph/conc/pool"
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

func (c *Client) FetchIssue(key string) (*ticket.Ticket, error) {
	// まずプロジェクトが存在するか確認
	if err := c.validateProject(); err != nil {
		return nil, err
	}
	issue, err := c.Get(context.Background(), key)
	if err != nil {
		return nil, err
	}
	return convert(issue)
}

// FetchIssues はJQLに基づいてJIRAチケットを取得します
func (c *Client) FetchIssues() (_ []*ticket.Ticket, err error) {
	defer derrors.Wrap(&err)
	// まずプロジェクトが存在するか確認
	if err := c.validateProject(); err != nil {
		return nil, err
	}

	// JQLクエリを作成
	jql := JQL(c.config.JQL)
	if jql == "" {
		jql = JQL(fmt.Sprintf("project = %s", c.config.Project.Key))
	}

	fetchIssues := func() (_ []*Issue, err error) {
		defer derrors.Wrap(&err)
		issues := make([]*Issue, 0, 10000)
		const limitRequestCount = 100 // 安全のための上限
		const bigNumber = 1
		ctx := context.Background()
		result, err := c.Search(ctx, jql, 0, bigNumber)
		if err != nil {
			return nil, err
		}
		if result.Total <= len(result.Issues) {
			// 1回のリクエストで全て取得できる場合
			return result.Issues, nil
		}
		issues = append(issues, result.Issues...)

		// > To find the maximum number of items that an operation could return, set maxResults to a large number—for example, over 1000—and if the returned value of maxResults is less than the requested value, the returned value is the maximum.
		// https://developer.atlassian.com/cloud/jira/platform/rest/v3/intro/#pagination
		maxResults := result.MaxResults // 上限の実際の値を取得すうる。(500にしても100でcapされた。)

		p := pool.NewWithResults[[]*Issue]().WithContext(ctx).WithMaxGoroutines(5)
		requestCount := 0
		for startAt := len(result.Issues); startAt < result.Total; startAt += maxResults {
			if requestCount >= limitRequestCount {
				break // 安全のため、リクエスト数の上限を設定
			}
			requestCount++
			p.Go(func(ctx context.Context) ([]*Issue, error) {
				fmt.Println(startAt, maxResults, jql)
				// ここでJQLを使ってJIRA APIに問い合わせる。
				result, err := c.Search(ctx, jql, startAt, maxResults)
				if err != nil {
					return nil, err
				}
				return result.Issues, nil
			})
		}
		listOfIssues, err := p.Wait()
		if err != nil {
			return nil, err
		}
		issues = append(issues, slices.Concat(listOfIssues...)...)
		return issues, nil
	}

	issues, err := fetchIssues()
	if err != nil {
		return nil, err
	}

	tickets := make([]*ticket.Ticket, 0, len(issues))
	for _, issue := range issues {
		ticket, err := convert(issue)
		if err != nil {
			return nil, err
		}
		tickets = append(tickets, ticket)
	}

	return tickets, nil
}

func convert(issue *Issue) (*ticket.Ticket, error) {
	ticket := &ticket.Ticket{
		Key:    issue.Key,
		Title:  issue.Fields.Summary,
		Type:   strings.ToLower(issue.Fields.IssueType.Name),
		Status: issue.Fields.Status.Name,
	}

	ticket.Body = adf.NewTranslator(issue.Fields.Description, adf.NewJiraMarkdownTranslator()).Translate()

	if issue.Fields.Parent != nil {
		ticket.ParentKey = issue.Fields.Parent.Key
	}
	if issue.Fields.Assignee != nil {
		ticket.Assignee = issue.Fields.Assignee.Name
	}
	if issue.Fields.Reporter != nil {
		ticket.Reporter = issue.Fields.Reporter.Name
	}

	// Parse timestamps
	createdAt, err := issue.Fields.CreatedAt()
	if err != nil {
		return nil, err
	}
	updatedAt, err := issue.Fields.UpdatedAt()
	if err != nil {
		return nil, err
	}
	ticket.CreatedAt = createdAt
	ticket.UpdatedAt = updatedAt
	return ticket, nil
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

type SearchResult struct {
	// StartAt    int      `json:"startAt"`
	MaxResults int      `json:"maxResults"`
	Total      int      `json:"total"`
	Issues     []*Issue `json:"issues"`
}

type Issue struct {
	Key    string      `json:"key"`
	Fields IssueFields `json:"fields"`
}

type IssueFields struct {
	Summary   string `json:"summary"`
	IssueType struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"issuetype"`
	Parent *struct {
		ID  string `json:"id"`
		Key string `json:"key"`
	}
	Status struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"status"`
	TimeOriginalEstimate *int     `json:"timeoriginalestimate"`
	Description          *adf.ADF `json:"description"`
	Assignee             *struct {
		AccountID    string `json:"accountId"`
		EmailAddress string `json:"emailAddress"`
		Name         string `json:"displayName"`
	} `json:"assignee"`
	Reporter *struct {
		AccountID    string `json:"accountId"`
		EmailAddress string `json:"emailAddress"`
		Name         string `json:"displayName"`
	} `json:"reporter"`
	Created string `json:"created"`
	Updated string `json:"updated"`
}

// 2025-06-01T19:06:22.513+0900
const jiraTimestampLayout = "2006-01-02T15:04:05.000-0700"

func (f *IssueFields) CreatedAt() (_ time.Time, err error) {
	defer derrors.Wrap(&err)
	createdAt, err := time.Parse(jiraTimestampLayout, f.Created)
	if err != nil {
		return time.Time{}, err
	}
	return createdAt, nil
}

func (f *IssueFields) UpdatedAt() (_ time.Time, err error) {
	defer derrors.Wrap(&err)
	updatedAt, err := time.Parse(jiraTimestampLayout, f.Updated)
	if err != nil {
		return time.Time{}, err
	}
	return updatedAt, nil
}

type JQL string

func (c *Client) Search(ctx context.Context, jql JQL, startAt, maxResults int) (_ *SearchResult, err error) {
	defer derrors.Wrap(&err)
	type Request struct {
		JQL        JQL      `json:"jql"`
		Fields     []string `json:"fields"`
		StartAt    int      `json:"startAt"`
		MaxResults int      `json:"maxResults"`
	}

	reqBody := Request{
		JQL: jql,
		Fields: []string{
			"issuetype",
			"timeoriginalestimate",
			"aggregatetimeoriginalestimate",
			"summary",
			"created",
			"status",
			"updated",
			"assignee",
			"issuetype",
			"description",
			"reporter",
			"parent",
		},
		StartAt:    startAt,
		MaxResults: maxResults,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}
	body := bytes.NewReader(jsonBody)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.config.Server+"/rest/api/3/search", body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth(c.config.Login, getAPIToken())

	client := &http.Client{}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("JIRA APIリクエストが失敗しました: " + resp.Status)
	}

	var result SearchResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result, nil
}

func (c *Client) Get(ctx context.Context, key string) (_ *Issue, err error) {
	defer derrors.Wrap(&err)
	jql := JQL(fmt.Sprintf(`key = "%s"`, key))
	result, err := c.Search(ctx, jql, 0, 1)
	if err != nil {
		return nil, err
	}
	if len(result.Issues) == 0 {
		return nil, fmt.Errorf("JIRAチケットが見つかりません: %s", key)
	}
	return result.Issues[0], nil
}
