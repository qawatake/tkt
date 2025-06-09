package cmd

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/qawatake/tkt/internal/config"
	"github.com/qawatake/tkt/internal/ticket"
	"github.com/spf13/cobra"
)

var grepCmd = &cobra.Command{
	Use:     "grep",
	Aliases: []string{"g"},
	Short:   "ローカルのファイルを全文検索します",
	Long:    `ローカルのファイルを全文検索します。チケットのkeyと内容を表示します。`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.LoadConfig()
		if err != nil {
			return fmt.Errorf("設定ファイルの読み込みに失敗しました: %v", err)
		}

		if cfg.Directory == "" {
			return fmt.Errorf("設定ファイルにdirectoryが設定されていません。tkt initで設定してください")
		}

		// マークダウンファイルを読み込み
		tickets, err := loadTickets(cfg.Directory)
		if err != nil {
			return fmt.Errorf("チケットの読み込みに失敗しました: %v", err)
		}

		if len(tickets) == 0 {
			return fmt.Errorf("チケットが見つかりません")
		}

		// Bubble Teaアプリを起動
		model := newGrepModel(tickets)
		p := tea.NewProgram(model, tea.WithAltScreen(), tea.WithMouseCellMotion())
		_, err = p.Run()
		return err
	},
}

type grepModel struct {
	tickets       []ticketItem
	filteredItems []ticketItem
	searchQuery   string
	cursor        int
	width         int
	height        int
}

type ticketItem struct {
	key     string
	title   string
	content string
}

func newGrepModel(tickets []*ticket.Ticket) grepModel {
	items := make([]ticketItem, len(tickets))
	for i, t := range tickets {
		items[i] = ticketItem{
			key:     t.Key,
			title:   t.Title,
			content: t.ToMarkdown(),
		}
	}

	return grepModel{
		tickets:       items,
		filteredItems: items,
		searchQuery:   "",
		cursor:        0,
	}
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
				m.searchQuery = m.searchQuery[:len(m.searchQuery)-1]
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
				// 一文字削除
				m.searchQuery = m.searchQuery[:len(m.searchQuery)-1]
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
			// escapeキーとqキーも含めて、すべての一文字入力を検索文字として扱う
			if len(msg.String()) == 1 || msg.String() == "esc" {
				if msg.String() == "esc" {
					// escapeキーは文字として扱えないので、別の文字に置き換える
					// または無視する
				} else {
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
	searchDisplay := searchStyle.Render(fmt.Sprintf("Search: %s_", m.searchQuery))
	helpText := helpStyle.Render(fmt.Sprintf("Found %d tickets • ctrl+p/n or ↑/↓:navigate • ctrl+h:delete • ctrl+k:clear • enter:select • ctrl+c:quit", len(m.filteredItems)))

	header := lipgloss.JoinVertical(lipgloss.Left,
		searchDisplay,
		helpText,
	)

	if len(m.filteredItems) == 0 {
		emptyMsg := lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			Render("No tickets found.")
		return lipgloss.JoinVertical(lipgloss.Left, header, emptyMsg)
	}

	// レイアウト計算
	headerHeight := lipgloss.Height(header)
	availableHeight := m.height - headerHeight
	leftWidth := m.width / 2
	rightWidth := m.width - leftWidth

	// 左ペイン（チケット一覧）
	leftPane := m.renderLeftPane(leftWidth-2, availableHeight-2)
	leftPaneStyled := borderStyle.
		Width(leftWidth - 2).
		Height(availableHeight - 2).
		Render(leftPane)

	// 右ペイン（チケット内容）
	rightPane := m.renderRightPane(rightWidth-2, availableHeight-2)
	rightPaneStyled := borderStyle.
		Width(rightWidth - 2).
		Height(availableHeight - 2).
		Render(rightPane)

	// 左右のペインを横に並べる
	body := lipgloss.JoinHorizontal(lipgloss.Top, leftPaneStyled, rightPaneStyled)

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
		keyPadded := fmt.Sprintf("%-12s", item.key)
		line := keyPadded

		// タイトルがある場合は表示
		if item.title != "" {
			line = fmt.Sprintf("%s %s", keyPadded, item.title)
		}

		// 幅に合わせてトリミング
		if len(line) > width {
			line = line[:width-3] + "..."
		}

		if i == m.cursor {
			line = selectedStyle.Width(width).Render(line)
		} else {
			line = lipgloss.NewStyle().Width(width).Render(line)
		}

		items = append(items, line)
	}

	// 残りの高さを空行で埋める
	for len(items) < height {
		items = append(items, lipgloss.NewStyle().Width(width).Render(""))
	}

	return strings.Join(items, "\n")
}

func (m grepModel) renderRightPane(width, height int) string {
	if len(m.filteredItems) == 0 || m.cursor >= len(m.filteredItems) {
		emptyMsg := lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			Width(width).
			Align(lipgloss.Center).
			Render("No ticket selected")

		var items []string
		items = append(items, emptyMsg)

		// 残りの高さを空行で埋める
		for len(items) < height {
			items = append(items, lipgloss.NewStyle().Width(width).Render(""))
		}

		return strings.Join(items, "\n")
	}

	content := m.filteredItems[m.cursor].content
	lines := strings.Split(content, "\n")

	var items []string
	for i := 0; i < height && i < len(lines); i++ {
		line := lines[i]
		if len(line) > width {
			line = line[:width-3] + "..."
		}

		// マークダウンのヘッダーをハイライト
		if strings.HasPrefix(line, "#") {
			line = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("33")).
				Render(line)
		}

		items = append(items, lipgloss.NewStyle().Width(width).Render(line))
	}

	// 残りの高さを空行で埋める
	for len(items) < height {
		items = append(items, lipgloss.NewStyle().Width(width).Render(""))
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
			tickets = append(tickets, t)
		}
		return nil
	})

	return tickets, err
}

func init() {
	rootCmd.AddCommand(grepCmd)
}
