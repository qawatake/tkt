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
	jiraClient    *jiralib.Client
	config        *config.Config
	sprintFieldID string // 動的に発見されたスプリントフィールドID
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

	client := &Client{
		jiraClient: jiraClient,
		config:     cfg,
	}

	// スプリントフィールドを動的に発見
	if err := client.discoverSprintField(); err != nil {
		verbose.Printf("スプリントフィールドの発見に失敗しました: %v\n", err)
		verbose.Printf("スプリント機能は無効になります\n")
		// エラーでもクライアント作成は続行（スプリント機能が使えないだけ）
	}

	return client, nil
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
	return c.convertWithSprint(issue)
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

	return c.fetchIssuesWithJQL(jql)
}

// FetchIssuesIncremental は最終フェッチ時刻以降に更新されたチケットのみを取得します
func (c *Client) FetchIssuesIncremental(lastFetch time.Time) (_ []*ticket.Ticket, err error) {
	defer derrors.Wrap(&err)
	// まずプロジェクトが存在するか確認
	if err := c.validateProject(); err != nil {
		return nil, err
	}

	// 基本のJQLクエリを作成
	baseJQL := c.config.JQL
	if baseJQL == "" {
		baseJQL = fmt.Sprintf("project = %s", c.config.Project.Key)
	}

	// 最終フェッチ時刻以降の更新条件を追加
	// JIRAのJQLでは yyyy/MM/dd HH:mm 形式を使用（分単位）
	lastFetchJQL := lastFetch.Format("2006/01/02 15:04")
	incrementalJQL := fmt.Sprintf("(%s) AND updated >= \"%s\"", baseJQL, lastFetchJQL)

	verbose.Printf("増分フェッチ用JQL: %s\n", incrementalJQL)

	return c.fetchIssuesWithJQL(JQL(incrementalJQL))
}

// fetchIssuesWithJQL は指定されたJQLでチケットを取得する共通処理です
func (c *Client) fetchIssuesWithJQL(jql JQL) (_ []*ticket.Ticket, err error) {
	defer derrors.Wrap(&err)

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
		ticket, err := c.convertWithSprint(issue)
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

	// スプリント情報は呼び出し元で設定される

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

// convertWithSprint はIssueをTicketに変換し、スプリント情報も設定します
func (c *Client) convertWithSprint(issue *Issue) (*ticket.Ticket, error) {
	tkt, err := convert(issue, c.config)
	if err != nil {
		return nil, err
	}

	// スプリント情報を動的に設定
	if c.sprintFieldID != "" {
		verbose.Printf("スプリントフィールドID: %s でデータ抽出を試行中...\n", c.sprintFieldID)
		sprintName := c.extractSprintNameFromIssue(issue)
		if sprintName != "" {
			verbose.Printf("スプリント名を発見: %s\n", sprintName)
		} else {
			verbose.Printf("スプリント名が見つかりませんでした\n")
		}
		tkt.SprintName = sprintName
	} else {
		verbose.Printf("スプリントフィールドIDが設定されていません\n")
	}

	return tkt, nil
}

// extractSprintNameFromIssue は動的にスプリント名を抽出します
func (c *Client) extractSprintNameFromIssue(issue *Issue) string {
	if c.sprintFieldID == "" {
		verbose.Printf("スプリントフィールドIDが空です\n")
		return ""
	}

	// CustomFieldsからスプリントフィールドを取得
	verbose.Printf("利用可能なカスタムフィールド数: %d\n", len(issue.Fields.CustomFields))

	sprintFieldValue, exists := issue.Fields.CustomFields[c.sprintFieldID]
	if !exists {
		verbose.Printf("スプリントフィールド %s が見つかりません\n", c.sprintFieldID)
		// デバッグ: 利用可能なカスタムフィールドを表示
		for key := range issue.Fields.CustomFields {
			if strings.HasPrefix(key, "customfield_") {
				verbose.Printf("  利用可能なカスタムフィールド: %s\n", key)
			}
		}
		return ""
	}

	verbose.Printf("スプリントフィールド値の型: %T, 値: %v\n", sprintFieldValue, sprintFieldValue)

	// nilの場合
	if sprintFieldValue == nil {
		verbose.Printf("スプリントフィールドがnullです\n")
		return ""
	}

	sprintField, ok := sprintFieldValue.([]interface{})
	if !ok {
		verbose.Printf("スプリントフィールドが配列ではありません\n")
		return ""
	}

	if len(sprintField) == 0 {
		verbose.Printf("スプリントフィールドが空です\n")
		return ""
	}

	verbose.Printf("スプリント数: %d\n", len(sprintField))

	// 最後のスプリント（現在のスプリント）を取得
	lastSprint, ok := sprintField[len(sprintField)-1].(map[string]interface{})
	if !ok {
		verbose.Printf("最後のスプリントがマップではありません。型: %T, 値: %v\n", sprintField[len(sprintField)-1], sprintField[len(sprintField)-1])
		return ""
	}

	verbose.Printf("最後のスプリント内容: %+v\n", lastSprint)

	if name, ok := lastSprint["name"].(string); ok {
		verbose.Printf("スプリント名抽出成功: %s\n", name)
		return name
	}

	verbose.Printf("nameフィールドが見つからないか型が不正です。利用可能なキー: %v\n", getKeys(lastSprint))
	return ""
}

// getKeys はマップのキー一覧を取得します
func getKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
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

	// スプリントフィールドの更新
	if err := c.addSprintFieldToUpdate(fields, ticket); err != nil {
		verbose.Printf("スプリントフィールドの設定に失敗しました: %v\n", err)
		// エラーでも他のフィールドの更新は続行
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

	// チケット作成用のフィールドを準備（カスタムフィールド対応のためmap形式）
	fields := map[string]interface{}{
		"project": map[string]string{
			"key": c.config.Project.Key,
		},
		"issuetype": map[string]string{
			"id": typeID,
		},
		"summary":     ticket.Title,
		"description": jiraDescription,
	}

	// 親チケットがある場合は設定
	if ticket.ParentKey != "" {
		fields["parent"] = map[string]string{
			"key": ticket.ParentKey,
		}
	}

	// スプリントが指定されている場合はカスタムフィールドに設定
	if ticket.SprintName != "" && c.sprintFieldID != "" && c.config.Board.ID != 0 {
		sprintID, err := c.findSprintIDByName(ticket.SprintName)
		if err != nil {
			verbose.Printf("スプリントIDの解決に失敗しました（作成時）: %v\n", err)
		} else if sprintID != 0 {
			verbose.Printf("作成時にスプリントフィールド %s を設定: %d\n", c.sprintFieldID, sprintID)
			fields[c.sprintFieldID] = sprintID
		}
	}

	// チケットを作成
	issue := map[string]interface{}{
		"fields": fields,
	}

	// デバッグ用：リクエストボディをログ出力
	if requestBody, marshalErr := json.MarshalIndent(issue, "", "  "); marshalErr == nil {
		verbose.Printf("JIRA Issue作成リクエスト:\n%s\n", string(requestBody))
	}

	// JSONボディを作成
	jsonBody, err := json.Marshal(issue)
	if err != nil {
		return nil, fmt.Errorf("リクエストボディの作成に失敗しました: %v", err)
	}

	// 直接HTTPリクエストを送信（カスタムフィールド対応のため）
	req, err := http.NewRequest(http.MethodPost,
		fmt.Sprintf("%s/rest/api/2/issue", c.config.Server),
		bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("HTTPリクエストの作成に失敗しました: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth(c.config.Login, getAPIToken())

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTPリクエストの送信に失敗しました: %v", err)
	}
	defer resp.Body.Close()

	// レスポンスボディを読み取り
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("レスポンスの読み取りに失敗しました: %v", err)
	}

	if resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("JIRAチケットの作成に失敗しました (status: %d): %s", resp.StatusCode, string(bodyBytes))
	}

	// レスポンスを解析して作成されたチケットのキーを取得
	var createResponse struct {
		Key string `json:"key"`
	}
	if err := json.Unmarshal(bodyBytes, &createResponse); err != nil {
		return nil, fmt.Errorf("作成レスポンスの解析に失敗しました: %v", err)
	}

	// 作成されたチケットをfetchして正しいフォーマットで返す
	createdTicket, err := c.FetchIssue(createResponse.Key)
	if err != nil {
		return nil, err
	}

	// スプリントは作成時にカスタムフィールドで直接設定済み
	verbose.Printf("チケット作成完了: %s (スプリント設定済み)\n", createResponse.Key)

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
	Created      string                 `json:"created"`
	Updated      string                 `json:"updated"`
	CustomFields map[string]interface{} `json:"-"` // カスタムフィールドを格納するためのマップ
}

// UnmarshalJSON はIssueFieldsの独自JSON解析を実装します
func (f *IssueFields) UnmarshalJSON(data []byte) error {
	// 既知のフィールドを定義した一時的な構造体
	type Alias IssueFields
	aux := &struct {
		*Alias
	}{
		Alias: (*Alias)(f),
	}

	// まず通常の構造体としてアンマーシャル
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	// 全フィールドをマップとしてアンマーシャル
	var allFields map[string]interface{}
	if err := json.Unmarshal(data, &allFields); err != nil {
		return err
	}

	// 既知のフィールドを除外してカスタムフィールドのみ抽出
	knownFields := map[string]bool{
		"summary": true, "issuetype": true, "parent": true, "status": true,
		"timeoriginalestimate": true, "description": true, "assignee": true,
		"reporter": true, "created": true, "updated": true,
	}

	f.CustomFields = make(map[string]interface{})
	for key, value := range allFields {
		if !knownFields[key] {
			f.CustomFields[key] = value
		}
	}

	return nil
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

	// スプリントフィールドが発見されている場合は追加
	if c.sprintFieldID != "" {
		fields = append(fields, c.sprintFieldID)
	}

	reqBody := Request{
		JQL:        jql,
		Fields:     fields,
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

	// レスポンスボディを読み取り、デバッグ出力
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	verbose.Printf("=== JIRA Search API Response ===\n")
	verbose.Printf("Status: %s\n", resp.Status)
	verbose.Printf("JQL: %s\n", jql)
	verbose.Printf("Body: %s\n", string(bodyBytes))
	verbose.Printf("================================\n")

	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("JIRA APIリクエストが失敗しました: " + resp.Status)
	}

	var result SearchResult
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
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

	// スプリントフィールドが発見されている場合は追加
	if c.sprintFieldID != "" {
		fields = append(fields, c.sprintFieldID)
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
		ticket, err := c.convertWithSprint(issue)
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

	// スプリントフィールドが発見されている場合は追加
	if c.sprintFieldID != "" {
		fields = append(fields, c.sprintFieldID)
	}

	reqBody := BulkFetchRequest{
		IssueIdsOrKeys: keys,
		Fields:         fields,
		FieldsByKeys:   false,
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

// GetBoardSprints は指定されたボードの全スプリントを取得します（ページネーション対応・並列処理）
func (c *Client) GetBoardSprints(boardID int) ([]Sprint, error) {
	return c.GetBoardSprintsWithContext(context.Background(), boardID)
}

// GetBoardSprintsWithContext は指定されたボードの全スプリントを取得します（ページネーション対応・並列処理）
func (c *Client) GetBoardSprintsWithContext(ctx context.Context, boardID int) ([]Sprint, error) {
	return c.getSprintsWithPagination(ctx, boardID, []string{})
}

// GetActiveAndFutureSprints は指定されたボードのアクティブと未来のスプリントを取得します（ページネーション対応・並列処理）
func (c *Client) GetActiveAndFutureSprints(boardID int) ([]Sprint, error) {
	return c.GetActiveAndFutureSprintsWithContext(context.Background(), boardID)
}

// GetActiveAndFutureSprintsWithContext は指定されたボードのアクティブと未来のスプリントを取得します（ページネーション対応・並列処理）
func (c *Client) GetActiveAndFutureSprintsWithContext(ctx context.Context, boardID int) ([]Sprint, error) {
	return c.getSprintsWithPagination(ctx, boardID, []string{"active", "future"})
}

// getSprintsPageWithTotal はスプリントの1ページを取得します（総数情報付き）
func (c *Client) getSprintsPageWithTotal(boardID int, startAt int, maxResults int, states []string) ([]Sprint, bool, int, error) {
	url := fmt.Sprintf("%s/rest/agile/1.0/board/%d/sprint", c.config.Server, boardID)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, false, 0, fmt.Errorf("HTTPリクエストの作成に失敗しました: %v", err)
	}

	q := req.URL.Query()
	q.Add("startAt", fmt.Sprintf("%d", startAt))
	q.Add("maxResults", fmt.Sprintf("%d", maxResults))
	if len(states) > 0 {
		q.Add("state", strings.Join(states, ","))
	}
	req.URL.RawQuery = q.Encode()

	req.SetBasicAuth(c.config.Login, getAPIToken())

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, false, 0, fmt.Errorf("HTTPリクエストの送信に失敗しました: %v", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, false, 0, fmt.Errorf("レスポンスの読み取りに失敗しました: %v", err)
	}

	// デバッグ用: APIレスポンスをダンプ
	verbose.Printf("DEBUG: Sprint API Response (boardID=%d, startAt=%d, maxResults=%d, states=%v):\n", boardID, startAt, maxResults, states)
	verbose.Printf("Status: %d\n", resp.StatusCode)
	verbose.Printf("Body: %s\n", string(bodyBytes))
	verbose.Printf("---\n")

	if resp.StatusCode != http.StatusOK {
		return nil, false, 0, fmt.Errorf("スプリント取得に失敗しました (status: %d): %s", resp.StatusCode, string(bodyBytes))
	}

	var response struct {
		Values     []Sprint `json:"values"`
		StartAt    int      `json:"startAt"`
		MaxResults int      `json:"maxResults"`
		Total      int      `json:"total"`
		IsLast     bool     `json:"isLast"`
	}

	if err := json.Unmarshal(bodyBytes, &response); err != nil {
		return nil, false, 0, fmt.Errorf("レスポンスの解析に失敗しました: %v", err)
	}

	return response.Values, response.IsLast, response.Total, nil
}

// getSprintsPage はスプリントの1ページを取得します
func (c *Client) getSprintsPage(boardID int, startAt int, maxResults int, states []string) ([]Sprint, bool, error) {
	sprints, isLast, _, err := c.getSprintsPageWithTotal(boardID, startAt, maxResults, states)
	return sprints, isLast, err
}

// GetActiveSprints は指定されたボードのアクティブなスプリントを取得します（ページネーション対応・並列処理）
func (c *Client) GetActiveSprints(boardID int) ([]Sprint, error) {
	return c.GetActiveSprintsWithContext(context.Background(), boardID)
}

// GetActiveSprintsWithContext は指定されたボードのアクティブなスプリントを取得します（ページネーション対応・並列処理）
func (c *Client) GetActiveSprintsWithContext(ctx context.Context, boardID int) ([]Sprint, error) {
	return c.getSprintsWithPagination(ctx, boardID, []string{"active"})
}

// getSprintsWithPagination はスプリントを並列処理でページネーション取得する汎用関数
func (c *Client) getSprintsWithPagination(ctx context.Context, boardID int, states []string) ([]Sprint, error) {
	const pageSize = 50

	// 最初のページを取得して全件数を把握
	firstPageSprints, isLast, total, err := c.getSprintsPageWithTotal(boardID, 0, pageSize, states)
	if err != nil {
		return nil, err
	}

	// 最初のページだけで終了の場合
	if isLast || total <= pageSize {
		return firstPageSprints, nil
	}

	// 必要なページ数を計算
	maxResults := pageSize
	totalPages := (total + maxResults - 1) / maxResults // 切り上げ除算

	// 結果を格納するスライス
	var allSprints []Sprint
	allSprints = append(allSprints, firstPageSprints...)

	// 2ページ目以降を並列で取得
	p := pool.NewWithResults[[]Sprint]().WithContext(ctx).WithMaxGoroutines(5)

	for page := 1; page < totalPages; page++ {
		currentStartAt := page * maxResults
		p.Go(func(ctx context.Context) ([]Sprint, error) {
			sprints, _, _, err := c.getSprintsPageWithTotal(boardID, currentStartAt, maxResults, states)
			if err != nil {
				return nil, err
			}
			return sprints, nil
		})
	}

	// 並列処理結果を取得
	results, err := p.Wait()
	if err != nil {
		return nil, err
	}

	// 結果をマージ
	for _, pageResults := range results {
		allSprints = append(allSprints, pageResults...)
	}

	return allSprints, nil
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

// addSprintFieldToUpdate はスプリントフィールドを更新フィールドに追加します
func (c *Client) addSprintFieldToUpdate(fields map[string]interface{}, ticket ticket.Ticket) error {
	// スプリント名が指定されていない場合は何もしない
	if ticket.SprintName == "" {
		verbose.Printf("スプリント名が指定されていないため、スプリント更新をスキップします\n")
		return nil
	}

	// スプリントフィールドIDが発見されていない場合は何もしない
	if c.sprintFieldID == "" {
		verbose.Printf("スプリントフィールドIDが見つからないため、スプリント更新をスキップします\n")
		return nil
	}

	// ボード設定がない場合は何もしない
	if c.config.Board.ID == 0 {
		verbose.Printf("ボード設定が見つからないため、スプリント更新をスキップします\n")
		return nil
	}

	// 目標スプリントのIDを解決
	targetSprintID, err := c.findSprintIDByName(ticket.SprintName)
	if err != nil {
		return fmt.Errorf("目標スプリントIDの解決に失敗しました: %v", err)
	}

	verbose.Printf("スプリントフィールド %s をスプリント '%s' (ID: %d) に設定します\n", c.sprintFieldID, ticket.SprintName, targetSprintID)

	// スプリントフィールドに直接スプリントIDを設定
	fields[c.sprintFieldID] = targetSprintID

	return nil
}

// discoverSprintField はJIRA APIからスプリントフィールドを動的に発見します
func (c *Client) discoverSprintField() error {
	req, err := http.NewRequest(http.MethodGet, c.config.Server+"/rest/api/3/field", nil)
	if err != nil {
		return fmt.Errorf("HTTPリクエストの作成に失敗しました: %v", err)
	}
	req.SetBasicAuth(c.config.Login, getAPIToken())

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("HTTPリクエストの送信に失敗しました: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("フィールド情報の取得に失敗しました (status: %d)", resp.StatusCode)
	}

	var fields []struct {
		ID     string `json:"id"`
		Name   string `json:"name"`
		Custom bool   `json:"custom"`
		Schema struct {
			Custom   string `json:"custom"`
			Type     string `json:"type"`
			Items    string `json:"items"`
			CustomID int    `json:"customId"`
		} `json:"schema"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&fields); err != nil {
		return fmt.Errorf("レスポンスの解析に失敗しました: %v", err)
	}

	// スプリントフィールドを検索
	for _, field := range fields {
		isSprintField := false

		// 複数の条件でスプリントフィールドを特定
		if field.Custom && field.Schema.Custom == "com.pyxis.greenhopper.jira:gh-sprint" {
			isSprintField = true
		} else if field.Custom && strings.ToLower(field.Name) == "sprint" {
			isSprintField = true
		} else if field.Custom && field.Schema.Type == "array" && field.Schema.Items == "json" {
			// スプリントフィールドの一般的な特徴: カスタム + 配列 + JSON項目
			if strings.Contains(strings.ToLower(field.Name), "sprint") {
				isSprintField = true
			}
		}

		if isSprintField {
			c.sprintFieldID = field.ID
			verbose.Printf("スプリントフィールドを発見しました: %s (%s) - Schema: %+v\n", field.ID, field.Name, field.Schema)
			return nil
		}
	}

	verbose.Printf("利用可能なカスタムフィールド:\n")
	for _, field := range fields {
		if field.Custom {
			verbose.Printf("  %s: %s (Schema: %+v)\n", field.ID, field.Name, field.Schema)
		}
	}

	return fmt.Errorf("スプリントフィールドが見つかりませんでした")
}

// DeleteIssue はJIRAからチケットを削除します
func (c *Client) DeleteIssue(issueKey string) error {
	req, err := http.NewRequest(http.MethodDelete,
		fmt.Sprintf("%s/rest/api/2/issue/%s", c.config.Server, issueKey), nil)
	if err != nil {
		return fmt.Errorf("HTTPリクエストの作成に失敗しました: %v", err)
	}

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
		return fmt.Errorf("JIRAチケットの削除に失敗しました (status: %d): %s", resp.StatusCode, errorMsg)
	}

	return nil
}
