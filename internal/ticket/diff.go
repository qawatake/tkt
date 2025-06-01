package ticket

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/sergi/go-diff/diffmatchpatch"
)

// DiffResult は差分の結果を表します
type DiffResult struct {
	Key      string
	FilePath string
	HasDiff  bool
	DiffText string
}

// CompareDirs はローカルディレクトリとキャッシュディレクトリの差分を検出します
func CompareDirs(localDir, cacheDir string) ([]DiffResult, error) {
	var results []DiffResult

	// ローカルディレクトリ内のファイルを走査
	localFiles, err := filepath.Glob(filepath.Join(localDir, "*.md"))
	if err != nil {
		return nil, fmt.Errorf("ローカルファイルの検索に失敗しました: %v", err)
	}

	for _, localFile := range localFiles {
		fileName := filepath.Base(localFile)
		cacheFile := filepath.Join(cacheDir, fileName)

		// ローカルファイルを読み込み
		localTicket, err := FromFile(localFile)
		if err != nil {
			return nil, fmt.Errorf("ローカルファイルの読み込みに失敗しました: %v", err)
		}

		// キャッシュファイルが存在するか確認
		if _, err := os.Stat(cacheFile); os.IsNotExist(err) {
			// キャッシュにないファイルは新規作成対象
			results = append(results, DiffResult{
				Key:      localTicket.Key,
				FilePath: localFile,
				HasDiff:  true,
				DiffText: fmt.Sprintf("新規チケット: %s", localTicket.Title),
			})
			continue
		}

		// キャッシュファイルを読み込み
		cacheTicket, err := FromFile(cacheFile)
		if err != nil {
			return nil, fmt.Errorf("キャッシュファイルの読み込みに失敗しました: %v", err)
		}

		// 差分を検出
		dmp := diffmatchpatch.New()
		diffs := dmp.DiffMain(cacheTicket.ToMarkdown(), localTicket.ToMarkdown(), false)
		diffText := dmp.DiffPrettyText(diffs)

		// 差分があるかどうか
		hasDiff := false
		for _, diff := range diffs {
			if diff.Type != diffmatchpatch.DiffEqual {
				hasDiff = true
				break
			}
		}

		results = append(results, DiffResult{
			Key:      localTicket.Key,
			FilePath: localFile,
			HasDiff:  hasDiff,
			DiffText: diffText,
		})
	}

	return results, nil
}
