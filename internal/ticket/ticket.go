package ticket

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	jiralib "github.com/andygrunwald/go-jira"
	"github.com/qawatake/tkt/pkg/markdown"
)

// Ticket はJIRAチケットのローカル表現です
type Ticket struct {
	Key              string    `yaml:"key"`
	ParentKey        string    `yaml:"parentKey"`
	Type             string    `yaml:"type"`
	Status           string    `yaml:"status"`
	Assignee         string    `yaml:"assignee"`
	Reporter         string    `yaml:"reporter"`
	CreatedAt        time.Time `yaml:"created_at"`
	UpdatedAt        time.Time `yaml:"updated_at"`
	OriginalEstimate Hour      `yaml:"original_estimate"`
	Title            string    `yaml:"-"`
	Body             string    `yaml:"-"`
	FilePath         string    `yaml:"-"`
}

type Hour float64

func NewHour(d time.Duration) Hour {
	return Hour(d) / Hour(time.Hour)
}

// FromIssue はJIRA APIのIssueからTicketを作成します
func FromIssue(issue *jiralib.Issue) *Ticket {
	// issueやfieldsがnilの場合の安全な処理
	if issue == nil || issue.Fields == nil {
		return &Ticket{}
	}

	// JIRA記法をMarkdownに変換
	var body string
	if issue.Fields.Description != "" {
		body = markdown.ConvertJiraToMarkdown(issue.Fields.Description)
	}

	var issueType, status string
	if issue.Fields.Type.Name != "" {
		issueType = strings.ToLower(issue.Fields.Type.Name)
	}
	if issue.Fields.Status.Name != "" {
		status = issue.Fields.Status.Name
	}

	ticket := &Ticket{
		Key:       issue.Key,
		Type:      issueType,
		Status:    status,
		CreatedAt: time.Time(issue.Fields.Created),
		UpdatedAt: time.Time(issue.Fields.Updated),
		Title:     issue.Fields.Summary,
		Body:      body,
	}

	// 親チケットがある場合は設定
	if issue.Fields.Parent != nil {
		ticket.ParentKey = issue.Fields.Parent.Key
	}

	// 担当者がある場合は設定
	if issue.Fields.Assignee != nil {
		ticket.Assignee = issue.Fields.Assignee.DisplayName
	}

	// レポーターがある場合は設定
	if issue.Fields.Reporter != nil {
		ticket.Reporter = issue.Fields.Reporter.DisplayName
	}

	return ticket
}

// ToMarkdown はチケットをマークダウン形式に変換します
func (t *Ticket) ToMarkdown() string {
	// フロントマターを作成
	frontMatterData := map[string]interface{}{}

	// keyがある場合のみ追加
	if t.Key != "" {
		frontMatterData["key"] = t.Key
	}

	// 必須項目
	frontMatterData["title"] = t.Title
	frontMatterData["type"] = t.Type

	// parentKeyがある場合のみ追加
	if t.ParentKey != "" {
		frontMatterData["parentKey"] = t.ParentKey
	}

	// readonly項目は値がある場合のみ追加
	if t.Status != "" {
		frontMatterData["status"] = t.Status
	}
	if t.Assignee != "" {
		frontMatterData["assignee"] = t.Assignee
	}
	if t.Reporter != "" {
		frontMatterData["reporter"] = t.Reporter
	}
	if !t.CreatedAt.IsZero() {
		frontMatterData["created_at"] = t.CreatedAt
	}
	if !t.UpdatedAt.IsZero() {
		frontMatterData["updated_at"] = t.UpdatedAt
	}
	if t.OriginalEstimate != 0 {
		frontMatterData["original_estimate"] = t.OriginalEstimate
	}

	frontMatter := markdown.CreateFrontMatter(frontMatterData)

	// マークダウン本文を作成
	return frontMatter + t.Body
}

// SaveToFile はチケットをファイルに保存します
func (t *Ticket) SaveToFile(dir string) (string, error) {
	// ディレクトリが存在しない場合は作成
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("ディレクトリの作成に失敗しました: %v", err)
	}

	// ファイル名を決定
	fileName := t.Key + ".md"
	if t.Key == "" {
		// キーがない場合はタイムスタンプからファイル名を生成
		timestamp := time.Now().Format("20060102-150405")
		fileName = fmt.Sprintf("TMP-%s.md", timestamp)
	}
	filePath := filepath.Join(dir, fileName)

	// マークダウンに変換
	content := t.ToMarkdown()

	// ファイルに書き込み
	if err := ioutil.WriteFile(filePath, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("ファイルの書き込みに失敗しました: %v", err)
	}

	t.FilePath = filePath
	return filePath, nil
}

// FromFile はファイルからチケットを読み込みます
func FromFile(filePath string) (*Ticket, error) {
	// ファイルを読み込み
	content, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("ファイルの読み込みに失敗しました: %v", err)
	}

	// フロントマターとコンテンツを分離
	frontMatter, body, err := markdown.ParseFrontMatter(string(content))
	if err != nil {
		return nil, fmt.Errorf("フロントマターの解析に失敗しました: %v", err)
	}

	// チケットを作成
	ticket := &Ticket{
		FilePath: filePath,
	}

	// フロントマターからフィールドを設定
	if key, ok := frontMatter["key"].(string); ok {
		ticket.Key = key
	}
	if title, ok := frontMatter["title"].(string); ok {
		ticket.Title = title
	}
	if parentKey, ok := frontMatter["parentKey"].(string); ok {
		ticket.ParentKey = parentKey
	}
	if typ, ok := frontMatter["type"].(string); ok {
		ticket.Type = typ
	}
	if status, ok := frontMatter["status"].(string); ok {
		ticket.Status = status
	}
	if assignee, ok := frontMatter["assignee"].(string); ok {
		ticket.Assignee = assignee
	}
	if reporter, ok := frontMatter["reporter"].(string); ok {
		ticket.Reporter = reporter
	}
	if createdAt, ok := frontMatter["created_at"].(time.Time); ok {
		ticket.CreatedAt = createdAt
	}
	if updatedAt, ok := frontMatter["updated_at"].(time.Time); ok {
		ticket.UpdatedAt = updatedAt
	}

	// 本文をそのまま設定
	ticket.Body = body

	return ticket, nil
}

// ToMarkdownWithoutReadonly はreadonly項目を除外したマークダウン形式を返します
func (t *Ticket) ToMarkdownWithoutReadonly() string {
	// readonly項目（key, status, assignee, reporter, created_at, updated_at）を除外したフロントマターを作成
	// titleはwritableなのでフロントマターに含める
	frontMatter := markdown.CreateFrontMatter(map[string]interface{}{
		"title":     t.Title,
		"parentKey": t.ParentKey,
		"type":      t.Type,
	})

	// フロントマターとbodyを結合
	return frontMatter + t.Body
}

// HasNonReadonlyDiff はreadonly項目以外に差分があるかチェックします
func (t *Ticket) HasNonReadonlyDiff(other *Ticket) bool {
	return t.ToMarkdownWithoutReadonly() != other.ToMarkdownWithoutReadonly()
}
