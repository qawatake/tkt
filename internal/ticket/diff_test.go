package ticket

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSeparateFrontMatter(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                string
		input               string
		expectedFrontMatter string
		expectedBody        string
	}{
		{
			name: "正常なfront matterとbody",
			input: `---
title: '[これは] いけるのか？'
type: タスク
---

[これはいけるのでしょうか] ふがふが

どうでしょうね。`,
			expectedFrontMatter: `---
title: '[これは] いけるのか？'
type: タスク
---
`,
			expectedBody: `
[これはいけるのでしょうか] ふがふが

どうでしょうね。`,
		},
		{
			name: "front matterのみ（bodyなし）",
			input: `---
title: 'テスト'
---`,
			expectedFrontMatter: `---
title: 'テスト'
---
`,
			expectedBody: "",
		},
		{
			name:                "front matterなし",
			input:               "これはbodyのみです\n\n日本語テスト",
			expectedFrontMatter: "",
			expectedBody:        "これはbodyのみです\n\n日本語テスト",
		},
		{
			name:                "空の文字列",
			input:               "",
			expectedFrontMatter: "",
			expectedBody:        "",
		},
		{
			name: "不正なfront matter（終了マーカーなし）",
			input: `---
title: 'テスト'
type: タスク

bodyテキスト`,
			expectedFrontMatter: "",
			expectedBody: `---
title: 'テスト'
type: タスク

bodyテキスト`,
		},
		{
			name: "front matterに日本語を含む",
			input: `---
title: '日本語タイトル [これは] テスト？'
assignee: '田中太郎'
---

日本語の本文です。
[リンク] テキスト`,
			expectedFrontMatter: `---
title: '日本語タイトル [これは] テスト？'
assignee: '田中太郎'
---
`,
			expectedBody: `
日本語の本文です。
[リンク] テキスト`,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			frontMatter, body := separateFrontMatter(tt.input)

			assert.Equal(t, tt.expectedFrontMatter, frontMatter, "front matterが期待値と一致しません")
			assert.Equal(t, tt.expectedBody, body, "bodyが期待値と一致しません")
		})
	}
}

func TestFormat(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name: "front matterが変換されないことを確認",
			input: `---
title: '[これは] いけるのか？'
type: タスク
---

[これはいけるのでしょうか] ふがふが`,
			expected: `---
title: '[これは] いけるのか？'
type: タスク
---
[これはいけるのでしょうか] ふがふが
`,
		},
		{
			name: "bodyのみが変換対象",
			input: `---
title: '[bracket] test'
---

Some **bold** text.
*italic* text.`,
			expected: `---
title: '[bracket] test'
---
Some **bold** text.
_italic_ text.
`,
		},
		{
			name: "front matterなしのMarkdown",
			input: `# 見出し

[リンクテキスト] 日本語テスト`,
			expected: `# 見出し
[リンクテキスト] 日本語テスト
`,
		},
		{
			name:     "空の文字列",
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := format(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFormat_JapaneseCharacterPreservation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		input       string
		description string
	}{
		{
			name: "日本語文字化け回帰テスト - front matterのタイトル",
			input: `---
title: '[これは] いけるのか？'
type: タスク
---

[これはいけるのでしょうか] ふがふが`,
			description: "front matterのtitleで日本語が文字化けしないこと",
		},
		{
			name: "日本語文字化け回帰テスト - body部分",
			input: `---
title: 'Test'
---

テスト文書です。
[これはテスト] ふがふがテスト
（これも）テストです。`,
			description: "body部分の日本語が文字化けしないこと",
		},
		{
			name: "エスケープ文字と日本語の組み合わせ",
			input: `---
title: 'Test'
---

\\[エスケープテスト\\] 日本語
\\(これも\\) テストです`,
			description: "エスケープ文字と日本語が正しく処理されること",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := format(tt.input)

			// front matterのtitleが変更されていないことを確認
			if strings.Contains(tt.input, "title: '[これは] いけるのか？'") {
				assert.Contains(t, result, "title: '[これは] いけるのか？'",
					"front matterのtitleが変更されてはいけません")
				assert.NotContains(t, result, "いけるの]？",
					"余分な]が追加されてはいけません")
			}

			// 日本語文字の文字化けがないことを確認
			assert.NotContains(t, result, "ãµããµã", "ふがふがが文字化けしてはいけません")
			assert.NotContains(t, result, "ãã¹ã", "テストが文字化けしてはいけません")
			assert.NotContains(t, result, "ã", "日本語文字化けの典型的なパターンが含まれてはいけません")

			// 正しい日本語が含まれることを確認
			if strings.Contains(tt.input, "ふがふが") {
				assert.Contains(t, result, "ふがふが", "日本語文字が正しく保持されていること")
			}
			if strings.Contains(tt.input, "テスト") {
				assert.Contains(t, result, "テスト", "日本語文字が正しく保持されていること")
			}
		})
	}
}

func TestFormat_EdgeCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "完全に空",
			input:    "",
			expected: "",
		},
		{
			name:     "改行のみ",
			input:    "\n\n\n",
			expected: "\n",
		},
		{
			name: "front matterのみ（body部分が空）",
			input: `---
title: 'Test'
---`,
			expected: `---
title: 'Test'
---
`,
		},
		{
			name: "不完全なfront matter",
			input: `---
title: 'Test'

bodyテキスト`,
			expected: `----
title: 'Test'

bodyテキスト
`,
		},
		{
			name: "複数のfront matterマーカー",
			input: `---
title: 'Test'
---

テキスト

---
別のマーカー
---`,
			expected: `---
title: 'Test'
---
テキスト----
別のマーカー

----
`,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := format(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
