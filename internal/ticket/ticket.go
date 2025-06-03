package ticket

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	jiralib "github.com/andygrunwald/go-jira"
	"github.com/gojira/gojira/pkg/markdown"
)

// Ticket はJIRAチケットのローカル表現です
type Ticket struct {
	Key       string    `yaml:"key"`
	ParentKey string    `yaml:"parentKey"`
	Type      string    `yaml:"type"`
	Status    string    `yaml:"status"`
	Assignee  string    `yaml:"assignee"`
	Reporter  string    `yaml:"reporter"`
	CreatedAt time.Time `yaml:"created_at"`
	UpdatedAt time.Time `yaml:"updated_at"`
	Title     string    `yaml:"-"`
	Body      string    `yaml:"-"`
	FilePath  string    `yaml:"-"`
}

// FromIssue はJIRA APIのIssueからTicketを作成します
func FromIssue(issue *jiralib.Issue) *Ticket {
	// JIRA記法をMarkdownに変換
	var body string
	if issue.Fields.Description != "" {
		body = markdown.ConvertJiraToMarkdown(issue.Fields.Description)
	}

	ticket := &Ticket{
		Key:       issue.Key,
		Type:      strings.ToLower(issue.Fields.Type.Name),
		Status:    issue.Fields.Status.Name,
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
	frontMatter := markdown.CreateFrontMatter(map[string]interface{}{
		"key":        t.Key,
		"parentKey":  t.ParentKey,
		"type":       t.Type,
		"status":     t.Status,
		"assignee":   t.Assignee,
		"reporter":   t.Reporter,
		"created_at": t.CreatedAt,
		"updated_at": t.UpdatedAt,
	})

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
		// キーがない場合はタイトルからファイル名を生成
		fileName = strings.ToLower(strings.ReplaceAll(t.Title, " ", "_")) + ".md"
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

	// 本文からタイトルと内容を抽出
	lines := strings.Split(body, "\n")
	if len(lines) > 0 && strings.HasPrefix(lines[0], "# ") {
		ticket.Title = strings.TrimPrefix(lines[0], "# ")
		ticket.Body = strings.Join(lines[1:], "\n")
	} else {
		ticket.Body = body
	}

	return ticket, nil
}
