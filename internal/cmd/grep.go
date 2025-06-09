package cmd

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
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
		p := tea.NewProgram(model, tea.WithAltScreen())
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
	return nil
}

func (m grepModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q", "esc":
			return m, tea.Quit

		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}

		case "down", "j":
			if m.cursor < len(m.filteredItems)-1 {
				m.cursor++
			}

		case "enter":
			return m, tea.Quit

		case "backspace":
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

		default:
			if len(msg.String()) == 1 {
				m.searchQuery += msg.String()
				m.filterItems()
				m.cursor = 0
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

func (m grepModel) View() string {
	if m.width == 0 || m.height == 0 {
		return "Loading..."
	}

	leftWidth := m.width / 2
	rightWidth := m.width - leftWidth - 1

	var s strings.Builder

	// ヘッダー
	s.WriteString(fmt.Sprintf("Search: %s\n", m.searchQuery))
	s.WriteString(fmt.Sprintf("Found %d tickets (use q to quit, j/k to navigate)\n", len(m.filteredItems)))
	s.WriteString(strings.Repeat("─", m.width) + "\n")

	if len(m.filteredItems) == 0 {
		s.WriteString("No tickets found.")
		return s.String()
	}

	// 左ペイン（チケット一覧）
	leftPane := m.renderLeftPane(leftWidth, m.height-3)
	rightPane := m.renderRightPane(rightWidth, m.height-3)

	// 左右のペインを並べて表示
	leftLines := strings.Split(leftPane, "\n")
	rightLines := strings.Split(rightPane, "\n")

	maxLines := len(leftLines)
	if len(rightLines) > maxLines {
		maxLines = len(rightLines)
	}

	for i := 0; i < maxLines; i++ {
		var left, right string
		if i < len(leftLines) {
			left = leftLines[i]
		}
		if i < len(rightLines) {
			right = rightLines[i]
		}

		// 左ペインの幅を調整
		if len(left) < leftWidth {
			left += strings.Repeat(" ", leftWidth-len(left))
		} else if len(left) > leftWidth {
			left = left[:leftWidth]
		}

		s.WriteString(left + "│" + right + "\n")
	}

	return s.String()
}

func (m grepModel) renderLeftPane(width, height int) string {
	var s strings.Builder

	start := 0
	if m.cursor >= height {
		start = m.cursor - height + 1
	}

	for i := start; i < start+height && i < len(m.filteredItems); i++ {
		item := m.filteredItems[i]
		prefix := "  "
		if i == m.cursor {
			prefix = "> "
		}

		line := fmt.Sprintf("%s%s", prefix, item.key)
		if len(line) > width {
			line = line[:width]
		}
		s.WriteString(line + "\n")
	}

	return s.String()
}

func (m grepModel) renderRightPane(width, height int) string {
	if len(m.filteredItems) == 0 || m.cursor >= len(m.filteredItems) {
		return ""
	}

	content := m.filteredItems[m.cursor].content
	lines := strings.Split(content, "\n")

	var s strings.Builder
	for i := 0; i < height && i < len(lines); i++ {
		line := lines[i]
		if len(line) > width {
			line = line[:width]
		}
		s.WriteString(line + "\n")
	}

	return s.String()
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
