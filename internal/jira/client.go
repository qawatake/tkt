package jira

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"slices"
	"strings"
	"time"

	jiralib "github.com/andygrunwald/go-jira"
	"github.com/k1LoW/errors"
	"github.com/qawatake/tkt/internal/adf"
	"github.com/qawatake/tkt/internal/config"
	"github.com/qawatake/tkt/internal/derrors"
	"github.com/qawatake/tkt/internal/md"
	"github.com/qawatake/tkt/internal/ticket"
	"github.com/qawatake/tkt/internal/verbose"
	"github.com/sourcegraph/conc/pool"
)

// Sprint はJIRAスプリントの情報を表します
type Sprint struct {
	ID           int    `json:"id"`
	Name         string `json:"name"`
	State        string `json:"state"`
	BoardID      int    `json:"originBoardId"`
	StartDate    string `json:"startDate"`
	EndDate      string `json:"endDate"`
	CompleteDate string `json:"completeDate"`
}

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

func (c *Client) FetchIssue(key string) (*ticket.Ticket, error) {
	// まずプロジェクトが存在するか確認
	if err := c.validateProject(); err != nil {
		return nil, err
	}
	issue, err := c.Get(context.Background(), key)
	if err != nil {
		return nil, err
	}
	return convert(issue, c.config)
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
		const bigNumber = 1000
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
				verbose.Println(startAt, maxResults, jql)
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
		ticket, err := convert(issue, c.config)
		if err != nil {
			return nil, err
		}
		tickets = append(tickets, ticket)
	}

	return tickets, nil
}

func convert(issue *Issue, cfg *config.Config) (*ticket.Ticket, error) {
	tkt := &ticket.Ticket{
		Key:    issue.Key,
		Title:  issue.Fields.Summary,
		Type:   strings.ToLower(issue.Fields.IssueType.Name),
		Status: issue.Fields.Status.Name,
		URL:    fmt.Sprintf("%s/browse/%s", cfg.Server, issue.Key),
	}

	tkt.Body = adf.NewTranslator(issue.Fields.Description, adf.NewJiraMarkdownTranslator()).Translate()

	if issue.Fields.Parent != nil {
		tkt.ParentKey = issue.Fields.Parent.Key
	}
	if issue.Fields.Assignee != nil {
		tkt.Assignee = issue.Fields.Assignee.Name
	}
	if issue.Fields.Reporter != nil {
		tkt.Reporter = issue.Fields.Reporter.Name
	}
	if issue.Fields.TimeOriginalEstimate != nil {
		tkt.OriginalEstimate = ticket.NewHour(time.Duration(*issue.Fields.TimeOriginalEstimate) * time.Second)
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
	tkt.CreatedAt = createdAt
	tkt.UpdatedAt = updatedAt
	return tkt, nil
}

// validateProject はプロジェクトが存在するか確認します
func (c *Client) validateProject() error {
	project, _, err := c.jiraClient.Project.Get(c.config.Project.Key)
	if err != nil {
		return fmt.Errorf("プロジェクト '%s' が見つかりません。設定ファイルのproject.keyを確認してください: %v", c.config.Project.Key, err)
	}

	verbose.Printf("プロジェクト確認: %s (%s)\n", project.Name, project.Key)
	return nil
}

// UpdateIssue はJIRAチケットを更新します
func (c *Client) UpdateIssue(ticket ticket.Ticket) error {
	// 更新用のフィールドを構築
	fields := make(map[string]interface{})

	// 基本フィールド
	if ticket.Title != "" {
		fields["summary"] = ticket.Title
	}
	if ticket.Body != "" {
		fields["description"] = md.ToJiraMD(ticket.Body)
	}
	if ticket.ParentKey != "" {
		fields["parent"] = map[string]string{"key": ticket.ParentKey}
	}
	if ticket.OriginalEstimate != 0 {
		fields["timetracking"] = map[string]interface{}{
			"originalEstimate": fmt.Sprintf("%.1fh", float64(ticket.OriginalEstimate)),
		}
	}

	updateData := map[string]interface{}{
		"fields": fields,
	}

	// JSON形式でリクエストボディを作成
	jsonBody, err := json.Marshal(updateData)
	if err != nil {
		return fmt.Errorf("リクエストボディの作成に失敗しました: %v", err)
	}

	// JIRA API v2を使用（JIRA記法をサポート）
	req, err := http.NewRequest(http.MethodPut,
		fmt.Sprintf("%s/rest/api/2/issue/%s", c.config.Server, ticket.Key),
		bytes.NewBuffer(jsonBody))
	if err != nil {
		return fmt.Errorf("HTTPリクエストの作成に失敗しました: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth(c.config.Login, getAPIToken())

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("HTTPリクエストの送信に失敗しました: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		bodyBytes, _ := io.ReadAll(resp.Body)
		errorMsg := string(bodyBytes)

		// エラーの詳細をログに出力
		verbose.Printf("JIRA更新エラー: %s\n", errorMsg)

		return fmt.Errorf("JIRAチケットの更新に失敗しました (status: %d): %s", resp.StatusCode, errorMsg)
	}

	// statusの更新（transition APIを使用）
	if ticket.Status != "" {
		err = c.updateIssueStatus(ticket.Key, ticket.Status)
		if err != nil {
			return fmt.Errorf("ステータスの更新に失敗しました: %v", err)
		}
	}

	return nil
}

// updateIssueStatus はJIRAチケットのステータスを更新します
func (c *Client) updateIssueStatus(issueKey, targetStatus string) error {
	// まず利用可能なトランジションを取得
	transitions, err := c.getAvailableTransitions(issueKey)
	if err != nil {
		return fmt.Errorf("利用可能なトランジション取得に失敗しました: %v", err)
	}

	// 目標ステータスに対応するトランジションIDを見つける
	var transitionID string
	var availableStatuses []string
	for _, transition := range transitions {
		availableStatuses = append(availableStatuses, transition.To.Name)
		if transition.To.Name == targetStatus {
			transitionID = transition.ID
			break
		}
	}

	if transitionID == "" {
		// 目標ステータスが見つからない場合はエラーとして返す
		return fmt.Errorf("ステータス '%s' への遷移が見つかりません。利用可能なステータス: %s",
			targetStatus, strings.Join(availableStatuses, ", "))
	}

	// トランジションを実行
	transitionData := map[string]interface{}{
		"transition": map[string]string{
			"id": transitionID,
		},
	}

	jsonBody, err := json.Marshal(transitionData)
	if err != nil {
		return fmt.Errorf("トランジションリクエストの作成に失敗しました: %v", err)
	}

	req, err := http.NewRequest(http.MethodPost,
		fmt.Sprintf("%s/rest/api/2/issue/%s/transitions", c.config.Server, issueKey),
		bytes.NewBuffer(jsonBody))
	if err != nil {
		return fmt.Errorf("HTTPリクエストの作成に失敗しました: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth(c.config.Login, getAPIToken())

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("HTTPリクエストの送信に失敗しました: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("ステータス更新に失敗しました (status: %d): %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

type Transition struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	To   struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"to"`
}

// getAvailableTransitions は指定されたチケットで利用可能なトランジションを取得します
func (c *Client) getAvailableTransitions(issueKey string) ([]Transition, error) {
	req, err := http.NewRequest(http.MethodGet,
		fmt.Sprintf("%s/rest/api/2/issue/%s/transitions", c.config.Server, issueKey),
		nil)
	if err != nil {
		return nil, fmt.Errorf("HTTPリクエストの作成に失敗しました: %v", err)
	}

	req.SetBasicAuth(c.config.Login, getAPIToken())

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTPリクエストの送信に失敗しました: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("トランジション取得に失敗しました (status: %d): %s", resp.StatusCode, string(bodyBytes))
	}

	var response struct {
		Transitions []Transition `json:"transitions"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("レスポンスの解析に失敗しました: %v", err)
	}

	return response.Transitions, nil
}

// CreateIssue は新しいJIRAチケットを作成します
func (c *Client) CreateIssue(ticket *ticket.Ticket) (*ticket.Ticket, error) {
	// チケットタイプIDを取得し、プロジェクトの妥当性も確認
	// createコマンドと同じフィルタリングロジックを使用
	typeID := ""
	var selectedType *config.IssueType

	verbose.Printf("チケットタイプ '%s' を検索中 (プロジェクト: %s, ID: %s)\n", ticket.Type, c.config.Project.Key, c.config.Project.ID)

	// プロジェクト固有のAPIから取得したすべてのIssue Typeを使用
	typeMap := make(map[string]config.IssueType)
	for _, issueType := range c.config.Issue.Types {
		verbose.Printf("  候補: %s (ID: %s)\n", issueType.Name, issueType.ID)
		typeMap[issueType.Name] = issueType
		verbose.Printf("    -> 追加\n")
	}

	// 指定されたタイプが見つかるかチェック
	if selectedIssueType, exists := typeMap[ticket.Type]; exists {
		selectedType = &selectedIssueType
		typeID = selectedType.ID
		verbose.Printf("選択されたタイプ: %s (ID: %s)\n", selectedType.Name, selectedType.ID)
	}

	if typeID == "" {
		verbose.Printf("利用可能なタイプ一覧:\n")
		for name, t := range typeMap {
			verbose.Printf("  - %s (ID: %s)\n", name, t.ID)
		}
		return nil, fmt.Errorf("チケットタイプが見つかりません: %s", ticket.Type)
	}

	// Markdown本文をJIRA記法に変換
	jiraDescription := md.ToJiraMD(ticket.Body)

	// チケット作成用のフィールドを準備
	fields := jiralib.IssueFields{
		Project: jiralib.Project{
			Key: c.config.Project.Key,
		},
		Type: jiralib.IssueType{
			ID: typeID,
		},
		Summary:     ticket.Title,
		Description: jiraDescription,
	}

	// 親チケットがある場合は設定
	if ticket.ParentKey != "" {
		fields.Parent = &jiralib.Parent{
			Key: ticket.ParentKey,
		}
	}

	// チケットを作成
	issue := jiralib.Issue{
		Fields: &fields,
	}

	// デバッグ用：リクエストボディをログ出力
	if requestBody, marshalErr := json.MarshalIndent(issue, "", "  "); marshalErr == nil {
		verbose.Printf("JIRA Issue作成リクエスト:\n%s\n", string(requestBody))
	}

	newIssue, response, err := c.jiraClient.Issue.Create(&issue)
	if err != nil {
		// HTTP レスポンスボディを読み取って詳細なエラー情報を取得
		var errorDetails string
		if response != nil && response.Body != nil {
			bodyBytes, readErr := io.ReadAll(response.Body)
			if readErr == nil {
				errorDetails = string(bodyBytes)
			}
		}
		if errorDetails != "" {
			return nil, fmt.Errorf("JIRAチケットの作成に失敗しました: %v\nレスポンス詳細: %s", err, errorDetails)
		}
		return nil, fmt.Errorf("JIRAチケットの作成に失敗しました: %v", err)
	}

	// 作成されたチケットをfetchして正しいフォーマットで返す
	createdTicket, err := c.FetchIssue(newIssue.Key)
	if err != nil {
		return nil, err
	}

	// スプリントが指定されている場合は、チケットをスプリントに追加
	if ticket.SprintName != "" {
		sprintID, err := c.findSprintIDByName(ticket.SprintName)
		if err != nil {
			verbose.Printf("スプリントIDの解決に失敗しました: %v\n", err)
		} else if sprintID != 0 {
			if err := c.AddIssueToSprint(newIssue.Key, sprintID); err != nil {
				verbose.Printf("スプリントへの追加に失敗しました: %v\n", err)
			}
		}
	}

	return createdTicket, nil
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

	fields := []string{
		"issuetype",
		"timeoriginalestimate",
		"aggregatetimeoriginalestimate",
		"summary",
		"created",
		"status",
		"updated",
		"assignee",
		"description",
		"reporter",
		"parent",
	}

	url := fmt.Sprintf("%s/rest/api/3/issue/%s?fields=%s", c.config.Server, key, strings.Join(fields, ","))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(c.config.Login, getAPIToken())

	client := &http.Client{}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("JIRAチケットが見つかりません: %s", key)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("JIRA APIリクエストが失敗しました: " + resp.Status)
	}

	var issue Issue
	if err := json.NewDecoder(resp.Body).Decode(&issue); err != nil {
		return nil, err
	}

	return &issue, nil
}

// BulkFetchIssues は複数のJIRAチケットを一括で取得します
func (c *Client) BulkFetchIssues(keys []string) (_ []*ticket.Ticket, err error) {
	defer derrors.Wrap(&err)
	if len(keys) == 0 {
		return []*ticket.Ticket{}, nil
	}

	// まずプロジェクトが存在するか確認
	if err := c.validateProject(); err != nil {
		return nil, err
	}

	const batchSize = 100 // JIRA Cloud APIの制限に基づく
	ctx := context.Background()

	// キーを適切なサイズに分割
	batches := make([][]string, 0, (len(keys)+batchSize-1)/batchSize)
	for i := 0; i < len(keys); i += batchSize {
		end := min(i+batchSize, len(keys))
		batches = append(batches, keys[i:end])
	}

	verbose.Printf("BulkFetchIssues: Total %d keys split into %d batches (max %d per batch)\n", len(keys), len(batches), batchSize)

	// 並列でバッチ処理
	p := pool.NewWithResults[[]*Issue]().WithContext(ctx).WithMaxGoroutines(5)
	for batchIndex, batch := range batches {
		batch := batch // ループ変数のキャプチャ
		batchIndex := batchIndex
		p.Go(func(ctx context.Context) ([]*Issue, error) {
			verbose.Printf("Starting batch %d: fetching %d issues (%v)\n", batchIndex+1, len(batch), batch)
			issues, err := c.bulkFetchBatch(ctx, batch)
			if err != nil {
				verbose.Printf("Batch %d failed: %v\n", batchIndex+1, err)
				return nil, err
			}
			verbose.Printf("Batch %d completed: successfully fetched %d issues\n", batchIndex+1, len(issues))
			return issues, nil
		})
	}

	listOfIssues, err := p.Wait()
	if err != nil {
		return nil, err
	}

	// 結果をフラット化
	allIssues := slices.Concat(listOfIssues...)

	// IssueからTicketに変換
	tickets := make([]*ticket.Ticket, 0, len(allIssues))
	for _, issue := range allIssues {
		ticket, err := convert(issue, c.config)
		if err != nil {
			return nil, err
		}
		tickets = append(tickets, ticket)
	}

	return tickets, nil
}

// bulkFetchBatch は単一バッチのチケットを取得します
func (c *Client) bulkFetchBatch(ctx context.Context, keys []string) (_ []*Issue, err error) {
	defer derrors.Wrap(&err)
	verbose.Printf("bulkFetchBatch: Making API call for keys: %v\n", keys)

	type BulkFetchRequest struct {
		IssueIdsOrKeys []string `json:"issueIdsOrKeys"`
		Fields         []string `json:"fields"`
		FieldsByKeys   bool     `json:"fieldsByKeys"`
	}

	type BulkFetchResponse struct {
		Issues []*Issue `json:"issues"`
		Errors []struct {
			IssueIDOrKey string `json:"issueIdOrKey"`
			ErrorMessage string `json:"errorMessage"`
		} `json:"errors"`
	}

	reqBody := BulkFetchRequest{
		IssueIdsOrKeys: keys,
		Fields: []string{
			"issuetype",
			"timeoriginalestimate",
			"aggregatetimeoriginalestimate",
			"summary",
			"created",
			"status",
			"updated",
			"assignee",
			"description",
			"reporter",
			"parent",
		},
		FieldsByKeys: false,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}
	body := bytes.NewReader(jsonBody)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.config.Server+"/rest/api/3/issue/bulkfetch", body)
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
		return nil, errors.New("JIRA Bulk Fetch APIリクエストが失敗しました: " + resp.Status)
	}

	var result BulkFetchResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	verbose.Printf("bulkFetchBatch: API response - got %d issues, %d errors\n", len(result.Issues), len(result.Errors))

	// エラーがある場合はログに出力（部分的な成功も許可）
	if len(result.Errors) > 0 {
		for _, apiErr := range result.Errors {
			verbose.Printf("Warning: Failed to fetch issue %s: %s\n", apiErr.IssueIDOrKey, apiErr.ErrorMessage)
		}
	}

	return result.Issues, nil
}

// GetBoardSprints は指定されたボードの全スプリントを取得します
func (c *Client) GetBoardSprints(boardID int) ([]Sprint, error) {
	url := fmt.Sprintf("%s/rest/agile/1.0/board/%d/sprint", c.config.Server, boardID)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("HTTPリクエストの作成に失敗しました: %v", err)
	}
	req.SetBasicAuth(c.config.Login, getAPIToken())

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTPリクエストの送信に失敗しました: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("スプリント取得に失敗しました (status: %d): %s", resp.StatusCode, string(bodyBytes))
	}

	var response struct {
		Values []Sprint `json:"values"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("レスポンスの解析に失敗しました: %v", err)
	}

	return response.Values, nil
}

// GetActiveSprints は指定されたボードのアクティブなスプリントを取得します
func (c *Client) GetActiveSprints(boardID int) ([]Sprint, error) {
	url := fmt.Sprintf("%s/rest/agile/1.0/board/%d/sprint?state=active", c.config.Server, boardID)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("HTTPリクエストの作成に失敗しました: %v", err)
	}
	req.SetBasicAuth(c.config.Login, getAPIToken())

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTPリクエストの送信に失敗しました: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("アクティブスプリント取得に失敗しました (status: %d): %s", resp.StatusCode, string(bodyBytes))
	}

	var response struct {
		Values []Sprint `json:"values"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("レスポンスの解析に失敗しました: %v", err)
	}

	return response.Values, nil
}

// AddIssueToSprint は指定されたチケットをスプリントに追加します
func (c *Client) AddIssueToSprint(issueKey string, sprintID int) error {
	url := fmt.Sprintf("%s/rest/agile/1.0/sprint/%d/issue", c.config.Server, sprintID)

	reqBody := struct {
		Issues []string `json:"issues"`
	}{
		Issues: []string{issueKey},
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("リクエストボディの作成に失敗しました: %v", err)
	}

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return fmt.Errorf("HTTPリクエストの作成に失敗しました: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth(c.config.Login, getAPIToken())

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("HTTPリクエストの送信に失敗しました: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("スプリントへのチケット追加に失敗しました (status: %d): %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

// findSprintIDByName はスプリント名からスプリントIDを解決します
func (c *Client) findSprintIDByName(sprintName string) (int, error) {
	// 設定からボードIDを取得
	if c.config.Board.ID == 0 {
		return 0, fmt.Errorf("ボード設定が見つかりません")
	}

	sprints, err := c.GetBoardSprints(c.config.Board.ID)
	if err != nil {
		return 0, fmt.Errorf("スプリント一覧の取得に失敗しました: %v", err)
	}

	for _, sprint := range sprints {
		if sprint.Name == sprintName {
			return sprint.ID, nil
		}
	}

	return 0, fmt.Errorf("スプリント '%s' が見つかりません", sprintName)
}
