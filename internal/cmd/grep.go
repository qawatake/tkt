package cmd

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/x/ansi"
	tty "github.com/mattn/go-tty"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/qawatake/tkt/internal/config"
	"github.com/qawatake/tkt/internal/derrors"
	"github.com/qawatake/tkt/internal/ticket"
	"github.com/spf13/cobra"
)

var grepCmd = &cobra.Command{
	Use:     "grep",
	Aliases: []string{"g"},
	Short:   "ローカルのファイルを全文検索します",
	Long:    `ローカルのファイルを全文検索します。チケットのkeyと内容を表示します。`,
	RunE: func(cmd *cobra.Command, args []string) (err error) {
		defer derrors.Wrap(&err)
		// デフォルトでキャッシュディレクトリを使用
		cacheDir, err := config.EnsureCacheDir()
		if err != nil {
			return fmt.Errorf("キャッシュディレクトリの取得に失敗しました: %v", err)
		}

		// マークダウンファイルを読み込み
		tickets, err := loadTickets(cacheDir)
		if err != nil {
			return fmt.Errorf("チケットの読み込みに失敗しました: %v", err)
		}

		if len(tickets) == 0 {
			return fmt.Errorf("チケットが見つかりません")
		}
		tty, err := tty.Open()
		if err != nil {
			return err
		}
		defer tty.Close()

		// Bubble Teaアプリを起動
		model, err := newGrepModel(tickets, cacheDir)
		if err != nil {
			return err
		}
		p := tea.NewProgram(model, tea.WithAltScreen(), tea.WithOutput(tty.Output()), tea.WithMouseCellMotion())
		_, err = p.Run()
		return err
	},
}

type grepModel struct {
	mdRenderer    *glamour.TermRenderer
	tickets       []ticketItem
	filteredItems []ticketItem
	searchQuery   string
	cursor        int
	width         int
	height        int
	configDir     string // 設定されたディレクトリを保持
}

type ticketItem struct {
	key     string
	title   string
	content string
}

func newGrepModel(tickets []*ticket.Ticket, configDir string) (_ grepModel, err error) {
	defer derrors.Wrap(&err)
	mdRenderer, err := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithEmoji(),
	)
	if err != nil {
		return grepModel{}, err
	}

	// updated_atの降順でソート
	sort.Slice(tickets, func(i, j int) bool {
		return tickets[i].UpdatedAt.After(tickets[j].UpdatedAt)
	})

	var items []ticketItem
	for _, t := range tickets {
		// 空のチケット（keyもtitleも空）をスキップ
		if t.Key == "" && t.Title == "" {
			continue
		}
		items = append(items, ticketItem{
			key:     t.Key,
			title:   t.Title,
			content: t.Body, // フロントマターを除いた本文のみ
		})
	}

	model := grepModel{
		mdRenderer:    mdRenderer,
		tickets:       items,
		filteredItems: items,
		searchQuery:   "",
		cursor:        0,
		configDir:     configDir,
	}

	// 初期状態で最初のファイルを確実に選択
	if len(items) > 0 {
		model.cursor = 0
	}

	return model, nil
}

func (m grepModel) Init() tea.Cmd {
	return tea.ClearScreen
}

func (m grepModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit

		case "enter":
			return m, tea.Quit

		case "up", "ctrl+p":
			if m.cursor > 0 {
				m.cursor--
			}

		case "down", "ctrl+n":
			if m.cursor < len(m.filteredItems)-1 {
				m.cursor++
			}

		case "backspace", "ctrl+h":
			if len(m.searchQuery) > 0 {
				// UTF-8対応：runeベースで最後の文字を削除
				runes := []rune(m.searchQuery)
				if len(runes) > 0 {
					m.searchQuery = string(runes[:len(runes)-1])
				}
				m.filterItems()
				if m.cursor >= len(m.filteredItems) {
					m.cursor = len(m.filteredItems) - 1
				}
				if m.cursor < 0 {
					m.cursor = 0
				}
			}

		case "ctrl+k":
			// 検索クエリをクリア
			m.searchQuery = ""
			m.filterItems()
			m.cursor = 0

		case "ctrl+u":
			// 検索クエリをクリア
			m.searchQuery = ""
			m.filterItems()
			m.cursor = 0

		case "ctrl+w":
			if len(m.searchQuery) > 0 {
				// 最後の単語を削除
				parts := strings.Fields(m.searchQuery)
				if len(parts) > 1 {
					m.searchQuery = strings.Join(parts[:len(parts)-1], " ") + " "
				} else {
					m.searchQuery = ""
				}
				m.filterItems()
				if m.cursor >= len(m.filteredItems) {
					m.cursor = len(m.filteredItems) - 1
				}
				if m.cursor < 0 {
					m.cursor = 0
				}
			}

		case "ctrl+d":
			if len(m.searchQuery) > 0 {
				// UTF-8対応：runeベースで一文字削除
				runes := []rune(m.searchQuery)
				if len(runes) > 0 {
					m.searchQuery = string(runes[:len(runes)-1])
				}
				m.filterItems()
				if m.cursor >= len(m.filteredItems) {
					m.cursor = len(m.filteredItems) - 1
				}
				if m.cursor < 0 {
					m.cursor = 0
				}
			}

		case "page_up":
			// ページアップ
			for i := 0; i < 10 && m.cursor > 0; i++ {
				m.cursor--
			}

		case "page_down":
			// ページダウン
			for i := 0; i < 10 && m.cursor < len(m.filteredItems)-1; i++ {
				m.cursor++
			}

		default:
			// 日本語を含む文字入力を検索文字として扱う
			switch msg.Type {
			case tea.KeyRunes:
				// 日本語などのマルチバイト文字に対応
				for _, r := range msg.Runes {
					m.searchQuery += string(r)
				}
				m.filterItems()
				m.cursor = 0
			default:
				// 従来の処理（ASCII文字）
				if len(msg.String()) == 1 && msg.String() != "esc" {
					m.searchQuery += msg.String()
					m.filterItems()
					m.cursor = 0
				}
			}
		}
	}

	return m, nil
}

func (m *grepModel) filterItems() {
	if m.searchQuery == "" {
		m.filteredItems = m.tickets
		// 初期状態では最初のファイルを選択
		if len(m.filteredItems) > 0 && m.cursor >= len(m.filteredItems) {
			m.cursor = 0
		}
		return
	}

	query := strings.ToLower(m.searchQuery)
	var filtered []ticketItem
	for _, item := range m.tickets {
		if strings.Contains(strings.ToLower(item.key), query) ||
			strings.Contains(strings.ToLower(item.title), query) ||
			strings.Contains(strings.ToLower(item.content), query) {
			filtered = append(filtered, item)
		}
	}
	m.filteredItems = filtered

	// フィルタリング後、カーソルが範囲外の場合は先頭に移動
	if len(m.filteredItems) > 0 && m.cursor >= len(m.filteredItems) {
		m.cursor = 0
	}
}

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("205"))

	searchStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("240")).
			Foreground(lipgloss.Color("230")).
			Padding(0, 1)

	selectedStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("57")).
			Foreground(lipgloss.Color("230"))

	borderStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("63"))

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241"))
)

func (m grepModel) View() string {
	// 最小限の表示を保証
	if m.width == 0 {
		m.width = 80
	}
	if m.height == 0 {
		m.height = 24
	}

	// ヘッダー部分
	header := searchStyle.Render(fmt.Sprintf("Search: %s_", m.searchQuery))

	if len(m.filteredItems) == 0 {
		emptyMsg := lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			Render("No tickets found.")
		return lipgloss.JoinVertical(lipgloss.Left, header, emptyMsg)
	}

	// レイアウト計算（3ペイン構成）
	headerHeight := lipgloss.Height(header)
	availableHeight := m.height - headerHeight
	leftWidth := m.width * 3 / 8                    // 左ペインを3/8に拡大
	rightWidth := m.width / 6                       // 右ペイン（フロントマター）を1/6に縮小
	centerWidth := m.width - leftWidth - rightWidth // 中央ペインは残り（約5/12）

	// 左ペイン（チケット一覧）
	leftPane := m.renderLeftPane(leftWidth-2, availableHeight-2)
	leftPaneStyled := borderStyle.
		Width(leftWidth - 2).
		Height(availableHeight - 2).
		Render(leftPane)

	// 中央ペイン（チケット内容）
	centerPane := lipgloss.NewStyle().
		MaxHeight(availableHeight - 2).
		Render(
			m.renderCenterPane(centerWidth-2, availableHeight-2),
		)
	centerPaneStyled := borderStyle.
		Width(centerWidth - 2).
		Height(availableHeight - 2).
		Render(centerPane)

	// 右ペイン（フロントマター）
	rightPane :=
		lipgloss.NewStyle().
			MaxHeight(availableHeight - 2).
			Render(
				m.renderRightPane(rightWidth-2, availableHeight-2),
			)
	rightPaneStyled := borderStyle.
		Width(rightWidth - 2).
		Height(availableHeight - 2).
		Render(rightPane)

	// 3つのペインを横に並べる
	body := lipgloss.JoinHorizontal(lipgloss.Top, leftPaneStyled, centerPaneStyled, rightPaneStyled)

	return lipgloss.JoinVertical(lipgloss.Left, header, body)
}

func (m grepModel) renderLeftPane(width, height int) string {
	var items []string

	start := 0
	if m.cursor >= height {
		start = m.cursor - height + 1
	}

	for i := start; i < start+height && i < len(m.filteredItems); i++ {
		item := m.filteredItems[i]

		// キーを固定幅（12文字）で左詰めパディング
		keyPadded := fmt.Sprintf("%-9s", item.key)
		line := keyPadded

		// タイトルがある場合は表示
		if item.title != "" {
			line = fmt.Sprintf("%s %s", keyPadded, item.title)
		}

		// 幅に合わせてトリミング
		line = ansi.TruncateWc(line, width, "…")

		if i == m.cursor {
			line = selectedStyle.Width(width).Render(line)
		} else {
			line = lipgloss.NewStyle().Width(width).Render(line)
		}

		items = append(items, line)
	}

	return strings.Join(items, "\n")
}

func (m grepModel) renderCenterPane(width, height int) string {
	if len(m.filteredItems) == 0 || m.cursor >= len(m.filteredItems) {
		emptyMsg := lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			Width(width).
			Align(lipgloss.Center).
			Render("No ticket selected")

		var items []string
		items = append(items, emptyMsg)

		return strings.Join(items, "\n")
	}

	content := m.filteredItems[m.cursor].content
	content, err := m.mdRenderer.Render(content)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		panic(err)
	}
	content = strings.TrimSpace(content)
	return lipgloss.NewStyle().Width(width - 2).MaxWidth(width).Render(content)
}

func (m grepModel) renderRightPane(width, height int) string {
	if len(m.filteredItems) == 0 || m.cursor >= len(m.filteredItems) {
		emptyMsg := lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			Width(width).
			Align(lipgloss.Center).
			Render("No metadata")

		var items []string
		items = append(items, emptyMsg)

		return strings.Join(items, "\n")
	}

	// 選択されたチケットのkeyからticketオブジェクトを取得
	selectedKey := m.filteredItems[m.cursor].key
	var selectedTicket *ticket.Ticket

	// チケット情報からフロントマター情報を取得
	for _, t := range m.tickets {
		if t.key == selectedKey {
			// ファイルからTicketを読み込んでフロントマター情報を取得
			if ticketData, err := ticket.FromFile(filepath.Join(m.configDir, t.key+".md")); err == nil {
				selectedTicket = ticketData
			}
			break
		}
	}

	var items []string

	if selectedTicket != nil {
		// フロントマター情報を表示
		frontmatterStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("33"))
		valueStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("252"))

		if selectedTicket.Key != "" {
			items = append(items, fmt.Sprintf("%s: %s",
				frontmatterStyle.Render("Key"),
				valueStyle.Render(selectedTicket.Key)))
		}

		if selectedTicket.Type != "" {
			items = append(items, fmt.Sprintf("%s: %s",
				frontmatterStyle.Render("Type"),
				valueStyle.Render(selectedTicket.Type)))
		}

		if selectedTicket.Status != "" {
			items = append(items, fmt.Sprintf("%s: %s",
				frontmatterStyle.Render("Status"),
				valueStyle.Render(selectedTicket.Status)))
		}

		if selectedTicket.Assignee != "" {
			items = append(items, fmt.Sprintf("%s: %s",
				frontmatterStyle.Render("Assignee"),
				valueStyle.Render(selectedTicket.Assignee)))
		}

		if selectedTicket.Reporter != "" {
			items = append(items, fmt.Sprintf("%s: %s",
				frontmatterStyle.Render("Reporter"),
				valueStyle.Render(selectedTicket.Reporter)))
		}

		// Parentを常に表示（設定されていない場合は"None"）
		if selectedTicket.ParentKey != "" {
			items = append(items, fmt.Sprintf("%s: %s",
				frontmatterStyle.Render("Parent"),
				valueStyle.Render(selectedTicket.ParentKey)))
		} else {
			items = append(items, fmt.Sprintf("%s: %s",
				frontmatterStyle.Render("Parent"),
				valueStyle.Render("None")))
		}

		// Original Estimateを0でも表示（設定されていない場合は"None"）
		if selectedTicket.OriginalEstimate > 0 {
			items = append(items, fmt.Sprintf("%s: %s",
				frontmatterStyle.Render("Estimate"),
				valueStyle.Render(fmt.Sprintf("%.1fh", float64(selectedTicket.OriginalEstimate)))))
		} else {
			items = append(items, fmt.Sprintf("%s: %s",
				frontmatterStyle.Render("Estimate"),
				valueStyle.Render("None")))
		}

		items = append(items, "") // 区切り線

		if !selectedTicket.CreatedAt.IsZero() {
			items = append(items, fmt.Sprintf("%s: %s",
				frontmatterStyle.Render("Created"),
				valueStyle.Render(selectedTicket.CreatedAt.Format("2006-01-02"))))
		}

		if !selectedTicket.UpdatedAt.IsZero() {
			items = append(items, fmt.Sprintf("%s: %s",
				frontmatterStyle.Render("Updated"),
				valueStyle.Render(selectedTicket.UpdatedAt.Format("2006-01-02"))))
		}
	} else {
		items = append(items, lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			Render("Metadata not available"))
	}

	// 各行を幅に合わせて調整（スタイル付き文字列はlipglossで処理）
	for i, item := range items {
		items[i] = lipgloss.NewStyle().Width(width).Render(item)
	}

	return strings.Join(items, "\n")
}

func loadTickets(dir string) ([]*ticket.Ticket, error) {
	var tickets []*ticket.Ticket

	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(path, ".md") {
			t, err := ticket.FromFile(path)
			if err != nil {
				// エラーは無視してスキップ
				return nil
			}
			// 有効なチケット（keyまたはtitleが存在）のみを追加
			if t.Key != "" || t.Title != "" {
				tickets = append(tickets, t)
			}
		}
		return nil
	})

	return tickets, err
}

func init() {
	rootCmd.AddCommand(grepCmd)
}
