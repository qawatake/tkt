package markdown

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// CreateFrontMatter はマップからYAMLフロントマターを作成します
func CreateFrontMatter(data map[string]interface{}) string {
	// マップをYAMLに変換
	yamlBytes, err := yaml.Marshal(data)
	if err != nil {
		return ""
	}

	// フロントマターフォーマットに整形
	return fmt.Sprintf("---\n%s---\n\n", string(yamlBytes))
}

// ParseFrontMatter はマークダウン文字列からフロントマターと本文を抽出します
func ParseFrontMatter(content string) (map[string]interface{}, string, error) {
	// フロントマターの開始と終了を検出
	if !strings.HasPrefix(content, "---\n") {
		return nil, content, nil
	}

	endIndex := strings.Index(content[4:], "---\n")
	if endIndex == -1 {
		return nil, content, fmt.Errorf("フロントマターの終了が見つかりません")
	}

	// フロントマターと本文を分離
	frontMatterStr := content[4 : 4+endIndex]
	body := content[4+endIndex+4:]

	// フロントマターをパース
	var frontMatter map[string]interface{}
	if err := yaml.Unmarshal([]byte(frontMatterStr), &frontMatter); err != nil {
		return nil, body, fmt.Errorf("フロントマターのパースに失敗しました: %v", err)
	}

	return frontMatter, body, nil
}
