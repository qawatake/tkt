package utils

import (
	"os"
)

// EnsureDir はディレクトリが存在することを確認し、存在しない場合は作成します
func EnsureDir(dir string) error {
	return os.MkdirAll(dir, 0755)
}
