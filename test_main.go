package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/gojira/gojira/internal/config"
	"github.com/gojira/gojira/internal/ticket"
)

// テスト用の簡易的なmain関数
// 実際のアプリケーションでは cmd/gojira/main.go が使用される
func main() {
	fmt.Println("gojira CLI テスト")

	// 設定ファイルのテスト
	fmt.Println("\n=== 設定ファイルのテスト ===")
	configDir := filepath.Join(os.Getenv("HOME"), ".config", "gojira")
	configFile := filepath.Join(configDir, "config.yml")
	
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		fmt.Printf("設定ファイルが見つかりません: %s\n", configFile)
		fmt.Println("テスト用の設定ファイルを作成します")
		
		// ディレクトリを作成
		if err := os.MkdirAll(configDir, 0755); err != nil {
			fmt.Printf("設定ディレクトリの作成に失敗しました: %v\n", err)
			os.Exit(1)
		}
		
		// テスト用の設定ファイルを作成
		testConfig := `auth_type: basic
login: test@example.com
server: https://example.atlassian.net
project:
  key: TEST
  type: next-gen
board:
  id: 1
  name: テストボード
  type: simple
epic:
  name: ""
  link: ""
issue:
  fields:
    custom:
      - name: Field1
        key: customfield_10001
        schema:
          datatype: string
  types:
    - id: "10001"
      name: タスク
      handle: Task
      subtask: false
    - id: "10002"
      name: エピック
      handle: Epic
      subtask: false
jql: "project = TEST"
timezone: Asia/Tokyo
`
		if err := os.WriteFile(configFile, []byte(testConfig), 0644); err != nil {
			fmt.Printf("設定ファイルの作成に失敗しました: %v\n", err)
			os.Exit(1)
		}
		
		fmt.Printf("テスト用の設定ファイルを作成しました: %s\n", configFile)
	}
	
	// 設定ファイルを読み込み
	cfg, err := config.LoadConfig()
	if err != nil {
		fmt.Printf("設定ファイルの読み込みに失敗しました: %v\n", err)
		os.Exit(1)
	}
	
	fmt.Printf("設定ファイルを読み込みました: %+v\n", cfg)
	
	// キャッシュディレクトリのテスト
	fmt.Println("\n=== キャッシュディレクトリのテスト ===")
	cacheDir, err := config.EnsureCacheDir()
	if err != nil {
		fmt.Printf("キャッシュディレクトリの確保に失敗しました: %v\n", err)
		os.Exit(1)
	}
	
	fmt.Printf("キャッシュディレクトリを確保しました: %s\n", cacheDir)
	
	// チケットのテスト
	fmt.Println("\n=== チケットのテスト ===")
	testTicket := &ticket.Ticket{
		Key:       "TEST-123",
		ParentKey: "TEST-100",
		Type:      "task",
		Status:    "To Do",
		Assignee:  "testuser",
		Reporter:  "reporter",
		CreatedAt: config.ParseTime("2023-01-01T12:00:00Z"),
		UpdatedAt: config.ParseTime("2023-01-02T12:00:00Z"),
		Title:     "テストチケット",
		Body:      "これはテストチケットです。",
	}
	
	// マークダウンに変換
	md := testTicket.ToMarkdown()
	fmt.Println("チケットをマークダウンに変換しました:")
	fmt.Println(md)
	
	// テスト用のディレクトリを作成
	testDir := "./tmp"
	if err := os.MkdirAll(testDir, 0755); err != nil {
		fmt.Printf("テストディレクトリの作成に失敗しました: %v\n", err)
		os.Exit(1)
	}
	
	// ファイルに保存
	filePath, err := testTicket.SaveToFile(testDir)
	if err != nil {
		fmt.Printf("チケットの保存に失敗しました: %v\n", err)
		os.Exit(1)
	}
	
	fmt.Printf("チケットをファイルに保存しました: %s\n", filePath)
	
	// ファイルから読み込み
	loadedTicket, err := ticket.FromFile(filePath)
	if err != nil {
		fmt.Printf("チケットの読み込みに失敗しました: %v\n", err)
		os.Exit(1)
	}
	
	fmt.Printf("チケットをファイルから読み込みました: %+v\n", loadedTicket)
	
	fmt.Println("\nすべてのテストが完了しました")
}
