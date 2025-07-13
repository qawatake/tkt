package utils

import (
	"os"
	"strings"
)

// EnsureDir はディレクトリが存在することを確認し、存在しない場合は作成します
func EnsureDir(dir string) error {
	return os.MkdirAll(dir, 0755)
}

// IsValidJIRAKey はJIRAキーの形式をチェックします (例: PRJ-123)
func IsValidJIRAKey(key string) bool {
	// プロジェクトキー-数字の形式
	parts := strings.Split(key, "-")
	if len(parts) != 2 {
		return false
	}

	// プロジェクトキーが英字のみ、数字部分が数字のみかチェック
	projectKey := parts[0]
	issueNumber := parts[1]

	// プロジェクトキーは英字のみ
	for _, r := range projectKey {
		if !((r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z')) {
			return false
		}
	}

	// 数字部分は数字のみ
	for _, r := range issueNumber {
		if !(r >= '0' && r <= '9') {
			return false
		}
	}

	return true
}
