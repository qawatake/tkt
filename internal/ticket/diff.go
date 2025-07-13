package ticket

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/filemode"
	"github.com/go-git/go-git/v5/plumbing/format/diff"
	"github.com/qawatake/tkt/internal/md"
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

	// 通常のファイルと削除済みファイル（ドットプレフィックス）を両方検索
	localFiles, err := filepath.Glob(filepath.Join(localDir, "*.md"))
	if err != nil {
		return nil, fmt.Errorf("ローカルファイルの検索に失敗しました: %v", err)
	}

	deletedFiles, err := filepath.Glob(filepath.Join(localDir, ".*.md"))
	if err != nil {
		return nil, fmt.Errorf("削除済みファイルの検索に失敗しました: %v", err)
	}

	// 削除済みファイルを処理
	for _, deletedFile := range deletedFiles {
		// 削除されたファイルを読み込み
		deletedTicket, err := FromFile(deletedFile)
		if err != nil {
			return nil, fmt.Errorf("削除済みファイルの読み込みに失敗しました: %v", err)
		}

		results = append(results, DiffResult{
			Key:      deletedTicket.Key,
			FilePath: deletedFile,
			HasDiff:  true,
			DiffText: fmt.Sprintf("削除されたチケット: %s", deletedTicket.Title),
		})
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

		// readonly項目以外に差分があるかチェック
		if !localTicket.HasNonReadonlyDiff(cacheTicket) {
			// readonly項目のみの変更の場合は差分なしとして扱う
			results = append(results, DiffResult{
				Key:      localTicket.Key,
				FilePath: localFile,
				HasDiff:  false,
				DiffText: "",
			})
			continue
		}

		// 差分を検出
		dmp := diffmatchpatch.New()
		dmp.DiffTimeout = 1 * time.Second // タイムアウトを設定
		fromRunes, toRunes, runesToLines := dmp.DiffLinesToRunes(format(cacheTicket.ToMarkdownWithoutReadonly()), format(localTicket.ToMarkdownWithoutReadonly()))
		diffs := dmp.DiffCharsToLines(dmp.DiffMainRunes(fromRunes, toRunes, false), runesToLines)
		chunks := make([]diff.Chunk, 0, len(diffs))
		for _, d := range diffs {
			chunk := newChunkFromDiff(d)
			chunks = append(chunks, chunk)
		}
		builder := strings.Builder{}
		unifiedEncoder := diff.NewUnifiedEncoder(&builder, diff.DefaultContextLines)
		unifiedEncoder.SetColor(diff.NewColorConfig())

		info, err := os.Stat(cacheFile)
		if err != nil {
			return nil, fmt.Errorf("キャッシュファイルの情報取得に失敗しました: %v", err)
		}
		fileMode, err := filemode.NewFromOSFileMode(info.Mode())
		if err != nil {
			return nil, err
		}
		from := &diffFile{
			fileMode: fileMode,
			relPath:  fileName,
			hash:     plumbing.ComputeHash(plumbing.BlobObject, []byte(format(cacheTicket.ToMarkdownWithoutReadonly()))),
		}
		info, err = os.Stat(localFile)
		if err != nil {
			return nil, fmt.Errorf("ローカルファイルの情報取得に失敗しました: %v", err)
		}
		fileMode, err = filemode.NewFromOSFileMode(info.Mode())
		if err != nil {
			return nil, err
		}
		to := &diffFile{
			fileMode: fileMode,
			relPath:  fileName,
			hash:     plumbing.ComputeHash(plumbing.BlobObject, []byte(format(localTicket.ToMarkdownWithoutReadonly()))),
		}

		patch := gitDiffPatch{
			filePatches: []diff.FilePatch{
				&filePatch{
					from:   from,
					to:     to,
					chunks: chunks,
				},
			},
		}

		err = unifiedEncoder.Encode(&patch)
		if err != nil {
			return nil, err
		}

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
			DiffText: builder.String(),
		})
	}

	return results, nil
}

// CommonMarkとして正規化しないと、パース結果が同じなのに差分があると検知されてしまいノイジーなので。
func format(body string) string {
	// front matterとbodyを分離
	frontMatter, content := separateFrontMatter(body)

	// bodyのみにJIRAのMarkdown変換を適用
	if content != "" {
		jmd := md.ToJiraMD(content)
		content = md.FromJiraMD(jmd)
	}

	// front matterとbodyを結合
	return frontMatter + content
}

// separateFrontMatter はMarkdownからfront matterとbodyを分離します
func separateFrontMatter(markdown string) (frontMatter, body string) {
	lines := strings.Split(markdown, "\n")

	// front matterの開始を確認
	if len(lines) < 3 || lines[0] != "---" {
		// front matterがない場合はそのまま返す
		return "", markdown
	}

	// front matterの終了位置を探す
	endIndex := -1
	for i := 1; i < len(lines); i++ {
		if lines[i] == "---" {
			endIndex = i
			break
		}
	}

	if endIndex == -1 {
		// front matterの終了が見つからない場合はそのまま返す
		return "", markdown
	}

	// front matter部分（区切り文字含む）
	frontMatterLines := lines[:endIndex+1]
	frontMatter = strings.Join(frontMatterLines, "\n")
	if len(frontMatterLines) > 0 {
		frontMatter += "\n"
	}

	// body部分
	if endIndex+1 < len(lines) {
		bodyLines := lines[endIndex+1:]
		body = strings.Join(bodyLines, "\n")
	}

	return frontMatter, body
}

// chezmoi diffを参考に。
// https://github.com/twpayne/chezmoi/blob/09214451c3904b77ec8d6303ff1ae221b75f93ce/internal/cmd/config.go#L1210
// https://github.com/twpayne/chezmoi/blob/09214451c3904b77ec8d6303ff1ae221b75f93ce/internal/chezmoi/diff.go#L67
type gitDiffPatch struct {
	filePatches []diff.FilePatch
	message     string
}

func (p *gitDiffPatch) FilePatches() []diff.FilePatch { return p.filePatches }
func (p *gitDiffPatch) Message() string               { return p.message }

type filePatch struct {
	from, to diff.File
	chunks   []diff.Chunk
}

var _ diff.FilePatch = (*filePatch)(nil)

func (f *filePatch) Chunks() []diff.Chunk        { return f.chunks }
func (f *filePatch) Files() (from, to diff.File) { return f.from, f.to }
func (f *filePatch) IsBinary() bool              { return false }
func (f *filePatch) FilePatches() []diff.FilePatch {
	return []diff.FilePatch{f}
}

type diffFile struct {
	fileMode filemode.FileMode
	relPath  string
	hash     plumbing.Hash
}

var _ diff.File = (*diffFile)(nil)

func (f *diffFile) Hash() plumbing.Hash     { return f.hash }
func (f *diffFile) Mode() filemode.FileMode { return f.fileMode }
func (f *diffFile) Path() string            { return f.relPath }

type diffChunk struct {
	content   string
	operation diff.Operation
}

var _ diff.Chunk = diffChunk{}

func (d diffChunk) Content() string {
	return d.content
}

func (d diffChunk) Type() diff.Operation {
	return d.operation
}

func newChunkFromDiff(d diffmatchpatch.Diff) diff.Chunk {
	var op diff.Operation
	switch d.Type {
	case diffmatchpatch.DiffInsert:
		op = diff.Add
	case diffmatchpatch.DiffDelete:
		op = diff.Delete
	default:
		op = diff.Equal
	}
	return diffChunk{content: d.Text, operation: op}
}
