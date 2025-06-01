package markdown

import (
	"regexp"
	"strings"
)

// ConvertJiraToMarkdown はJIRA記法をMarkdownに変換します
func ConvertJiraToMarkdown(input string) string {
	if input == "" {
		return input
	}

	// 改行を統一
	content := strings.ReplaceAll(input, "\r\n", "\n")
	content = strings.ReplaceAll(content, "\r", "\n")

	// コードブロック変換（最初に処理してコードブロック内のマークアップを保護）
	content = convertCodeBlocks(content)

	// 見出し変換
	content = convertHeadings(content)

	// リスト変換（見出し変換の後に処理）
	content = convertLists(content)

	// テキスト装飾変換
	content = convertTextFormatting(content)

	// パネル変換
	content = convertPanels(content)

	// 引用変換
	content = convertQuotes(content)

	// リンク変換
	content = convertLinks(content)

	// テーブル変換
	content = convertTables(content)

	return content
}

// convertHeadings は見出しを変換します (h1. -> #, h2. -> ## など)
func convertHeadings(content string) string {
	headings := map[string]string{
		"h1.": "#",
		"h2.": "##",
		"h3.": "###",
		"h4.": "####",
		"h5.": "#####",
		"h6.": "######",
	}

	for jira, md := range headings {
		// 行の開始からh1.などで始まる場合のみ変換
		pattern := `(?m)^` + regexp.QuoteMeta(jira) + `\s+(.+)$`
		re := regexp.MustCompile(pattern)
		content = re.ReplaceAllString(content, md+" $1")
	}

	return content
}

// convertCodeBlocks はコードブロックを変換します
func convertCodeBlocks(content string) string {
	// {code:language}...{code} -> ```language...```
	codeBlockRe := regexp.MustCompile(`(?s)\{code(?::([^}]*))?\}(.*?)\{code\}`)
	content = codeBlockRe.ReplaceAllStringFunc(content, func(match string) string {
		parts := codeBlockRe.FindStringSubmatch(match)
		language := ""
		if len(parts) > 1 && parts[1] != "" {
			language = parts[1]
		}
		code := ""
		if len(parts) > 2 {
			code = strings.TrimSpace(parts[2])
		}
		return "```" + language + "\n" + code + "\n```"
	})

	// {noformat}...{noformat} -> ```...```
	noFormatRe := regexp.MustCompile(`(?s)\{noformat\}(.*?)\{noformat\}`)
	content = noFormatRe.ReplaceAllString(content, "```\n$1\n```")

	return content
}

// convertTextFormatting はテキスト装飾を変換します
func convertTextFormatting(content string) string {
	// 太字: *text* -> **text** (ただし、リストマーカーでない場合のみ)
	boldRe := regexp.MustCompile(`(?m)(?:^|[^*\n])\*([^*\n]+)\*(?:[^*\n]|$)`)
	content = boldRe.ReplaceAllStringFunc(content, func(match string) string {
		// マッチした内容を分析
		re := regexp.MustCompile(`(.*?)\*([^*\n]+)\*(.*)`)
		parts := re.FindStringSubmatch(match)
		if len(parts) == 4 {
			return parts[1] + "**" + parts[2] + "**" + parts[3]
		}
		return match
	})

	// 斜体: _text_ -> *text*
	italicRe := regexp.MustCompile(`_([^_\n]+)_`)
	content = italicRe.ReplaceAllString(content, "*$1*")

	// 下線: +text+ -> __text__ (Markdownに下線はないので太字で代用)
	underlineRe := regexp.MustCompile(`\+([^+\n]+)\+`)
	content = underlineRe.ReplaceAllString(content, "__$1__")

	// 取り消し線: -text- -> ~~text~~ (ただし、リストマーカーでない場合のみ)
	strikeRe := regexp.MustCompile(`(?m)(?:^|[^-\n])-([^-\n]+)-(?:[^-\n]|$)`)
	content = strikeRe.ReplaceAllStringFunc(content, func(match string) string {
		re := regexp.MustCompile(`(.*?)-([^-\n]+)-(.*)`)
		parts := re.FindStringSubmatch(match)
		if len(parts) == 4 {
			return parts[1] + "~~" + parts[2] + "~~" + parts[3]
		}
		return match
	})

	// インラインコード: {{text}} -> `text`
	inlineCodeRe := regexp.MustCompile(`\{\{([^}]+)\}\}`)
	content = inlineCodeRe.ReplaceAllString(content, "`$1`")

	// 上付き文字: ^text^ -> text (Markdownには上付きがないので削除)
	supRe := regexp.MustCompile(`\^([^^]+)\^`)
	content = supRe.ReplaceAllString(content, "$1")

	// 下付き文字: ~text~ -> text (Markdownには下付きがないので削除)
	subRe := regexp.MustCompile(`~([^~]+)~`)
	content = subRe.ReplaceAllString(content, "$1")

	return content
}

// convertLists はリストを変換します
func convertLists(content string) string {
	lines := strings.Split(content, "\n")
	var result []string

	for _, line := range lines {
		// Markdownの見出し（#で始まる）はスキップ
		if strings.HasPrefix(line, "#") {
			result = append(result, line)
			continue
		}

		// 順序付きリスト: # item -> 1. item（見出しでない場合のみ）
		if regexp.MustCompile(`^# [^#]`).MatchString(line) {
			result = append(result, "1. "+strings.TrimPrefix(line, "# "))
		} else if regexp.MustCompile(`^## [^#]`).MatchString(line) {
			result = append(result, "   1. "+strings.TrimPrefix(line, "## "))
		} else if regexp.MustCompile(`^### [^#]`).MatchString(line) {
			result = append(result, "      1. "+strings.TrimPrefix(line, "### "))
			// 順序なしリスト: * item -> - item
		} else if strings.HasPrefix(line, "* ") {
			result = append(result, "- "+strings.TrimPrefix(line, "* "))
		} else if strings.HasPrefix(line, "** ") {
			result = append(result, "  - "+strings.TrimPrefix(line, "** "))
		} else if strings.HasPrefix(line, "*** ") {
			result = append(result, "    - "+strings.TrimPrefix(line, "*** "))
		} else {
			result = append(result, line)
		}
	}

	return strings.Join(result, "\n")
}

// convertPanels はパネルを変換します
func convertPanels(content string) string {
	// {panel:title=Title}...{panel} -> ### Title\n...\n---
	panelRe := regexp.MustCompile(`(?s)\{panel(?::title=([^}]*))?\}(.*?)\{panel\}`)
	content = panelRe.ReplaceAllStringFunc(content, func(match string) string {
		parts := panelRe.FindStringSubmatch(match)
		title := ""
		if len(parts) > 1 && parts[1] != "" {
			title = "### " + parts[1] + "\n\n"
		}
		body := ""
		if len(parts) > 2 {
			body = strings.TrimSpace(parts[2])
		}
		return title + body + "\n\n---"
	})

	return content
}

// convertQuotes は引用を変換します
func convertQuotes(content string) string {
	// {quote}...{quote} -> > ...
	quoteRe := regexp.MustCompile(`(?s)\{quote\}(.*?)\{quote\}`)
	content = quoteRe.ReplaceAllStringFunc(content, func(match string) string {
		parts := quoteRe.FindStringSubmatch(match)
		if len(parts) > 1 {
			lines := strings.Split(strings.TrimSpace(parts[1]), "\n")
			var quoted []string
			for _, line := range lines {
				quoted = append(quoted, "> "+line)
			}
			return strings.Join(quoted, "\n")
		}
		return match
	})

	// bq. line -> > line
	bqRe := regexp.MustCompile(`(?m)^bq\.\s+(.+)$`)
	content = bqRe.ReplaceAllString(content, "> $1")

	return content
}

// convertLinks はリンクを変換します
func convertLinks(content string) string {
	// [text|url] -> [text](url)
	linkRe := regexp.MustCompile(`\[([^|\]]+)\|([^\]]+)\]`)
	content = linkRe.ReplaceAllString(content, "[$1]($2)")

	// [url] -> [url](url)
	simpleLinkRe := regexp.MustCompile(`\[([^|\]]+)\]`)
	content = simpleLinkRe.ReplaceAllString(content, "[$1]($1)")

	return content
}

// convertTables はテーブルを変換します
func convertTables(content string) string {
	lines := strings.Split(content, "\n")
	var result []string
	inTable := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// JIRAテーブル行の検出: ||header1||header2|| または |cell1|cell2|
		if strings.HasPrefix(trimmed, "||") && strings.HasSuffix(trimmed, "||") {
			// ヘッダー行
			headers := strings.Split(strings.Trim(trimmed, "|"), "||")
			var cleanHeaders []string
			for _, header := range headers {
				cleanHeaders = append(cleanHeaders, strings.TrimSpace(header))
			}

			if len(cleanHeaders) > 0 {
				// Markdownテーブルヘッダー
				result = append(result, "| "+strings.Join(cleanHeaders, " | ")+" |")

				// セパレータ行
				var separators []string
				for range cleanHeaders {
					separators = append(separators, "---")
				}
				result = append(result, "| "+strings.Join(separators, " | ")+" |")

				inTable = true
			}
		} else if strings.HasPrefix(trimmed, "|") && strings.HasSuffix(trimmed, "|") && inTable {
			// データ行
			cells := strings.Split(strings.Trim(trimmed, "|"), "|")
			var cleanCells []string
			for _, cell := range cells {
				cleanCells = append(cleanCells, strings.TrimSpace(cell))
			}

			if len(cleanCells) > 0 {
				result = append(result, "| "+strings.Join(cleanCells, " | ")+" |")
			}
		} else {
			// テーブル以外の行
			if inTable {
				// テーブル終了
				inTable = false
			}
			result = append(result, line)
		}
	}

	return strings.Join(result, "\n")
}

// ConvertMarkdownToJira はMarkdownをJIRA記法に変換します
func ConvertMarkdownToJira(input string) string {
	if input == "" {
		return input
	}

	// 改行を統一
	content := strings.ReplaceAll(input, "\r\n", "\n")
	content = strings.ReplaceAll(content, "\r", "\n")

	// 見出し変換 (# -> h1., ## -> h2. など)
	content = convertMarkdownHeadings(content)

	// リスト変換
	content = convertMarkdownLists(content)

	// テキスト装飾変換
	content = convertMarkdownTextFormatting(content)

	// コードブロック変換
	content = convertMarkdownCodeBlocks(content)

	// リンク変換
	content = convertMarkdownLinks(content)

	return content
}

// convertMarkdownHeadings はMarkdownの見出しをJIRA記法に変換します
func convertMarkdownHeadings(content string) string {
	headings := map[string]string{
		"######": "h6.",
		"#####":  "h5.",
		"####":   "h4.",
		"###":    "h3.",
		"##":     "h2.",
		"#":      "h1.",
	}

	lines := strings.Split(content, "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		for md, jira := range headings {
			if strings.HasPrefix(trimmed, md+" ") {
				title := strings.TrimSpace(strings.TrimPrefix(trimmed, md))
				lines[i] = jira + " " + title
				break
			}
		}
	}

	return strings.Join(lines, "\n")
}

// convertMarkdownLists はMarkdownのリストをJIRA記法に変換します
func convertMarkdownLists(content string) string {
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		// 順序なしリスト: - や * を * に変換
		if strings.HasPrefix(strings.TrimSpace(line), "- ") {
			indentMatch := regexp.MustCompile(`^(\s*)`).FindStringSubmatch(line)
			indent := ""
			if len(indentMatch) > 1 {
				indent = indentMatch[1]
			}
			text := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "-"))
			lines[i] = indent + "* " + text
		}

		// 順序付きリスト: 1. を # に変換
		if matched, _ := regexp.MatchString(`^\s*\d+\.\s`, line); matched {
			re := regexp.MustCompile(`^(\s*)\d+\.\s(.*)$`)
			matches := re.FindStringSubmatch(line)
			if len(matches) == 3 {
				indent := matches[1]
				text := matches[2]
				lines[i] = indent + "# " + text
			}
		}
	}

	return strings.Join(lines, "\n")
}

// convertMarkdownTextFormatting はMarkdownのテキスト装飾をJIRA記法に変換します
func convertMarkdownTextFormatting(content string) string {
	// 太字: **text** -> *text*
	content = regexp.MustCompile(`\*\*([^*]+)\*\*`).ReplaceAllString(content, "*$1*")

	// 斜体: *text* -> _text_
	content = regexp.MustCompile(`\*([^*]+)\*`).ReplaceAllString(content, "_$1_")

	// インラインコード: `text` -> {{text}}
	content = regexp.MustCompile("`([^`]+)`").ReplaceAllString(content, "{{$1}}")

	return content
}

// convertMarkdownCodeBlocks はMarkdownのコードブロックをJIRA記法に変換します
func convertMarkdownCodeBlocks(content string) string {
	// ```language または ``` で始まるコードブロック
	re := regexp.MustCompile("(?s)```(\\w+)?\\n(.*?)\\n```")
	content = re.ReplaceAllStringFunc(content, func(match string) string {
		submatch := re.FindStringSubmatch(match)
		if len(submatch) >= 3 {
			language := submatch[1]
			code := submatch[2]
			if language != "" {
				return "{code:" + language + "}\n" + code + "\n{code}"
			}
			return "{code}\n" + code + "\n{code}"
		}
		return match
	})

	return content
}

// convertMarkdownLinks はMarkdownのリンクをJIRA記法に変換します
func convertMarkdownLinks(content string) string {
	// [text](url) -> [text|url]
	content = regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`).ReplaceAllString(content, "[$1|$2]")

	return content
}
