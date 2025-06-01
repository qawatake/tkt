package utils

import (
	"os"
	"path/filepath"
)

// EnsureDir はディレクトリが存在することを確認し、存在しない場合は作成します
func EnsureDir(dir string) error {
	return os.MkdirAll(dir, 0755)
}

// FileExists はファイルが存在するかどうかを確認します
func FileExists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}

// GetFilesInDir はディレクトリ内のファイルパスを取得します
func GetFilesInDir(dir string, pattern string) ([]string, error) {
	return filepath.Glob(filepath.Join(dir, pattern))
}
