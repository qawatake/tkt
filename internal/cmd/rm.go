package cmd

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	tty "github.com/mattn/go-tty"
	"github.com/qawatake/tkt/internal/config"
	"github.com/qawatake/tkt/internal/derrors"
	"github.com/qawatake/tkt/internal/ticket"
	"github.com/qawatake/tkt/internal/ui"
	"github.com/spf13/cobra"
)

var rmCmd = &cobra.Command{
	Use:     "rm [ticket-key...]",
	Aliases: []string{"remove", "delete"},
	Short:   "ローカルのチケットを削除します",
	Long:    `ローカルのチケットを削除します。引数なしの場合はインタラクティブに選択、引数ありの場合は指定されたチケットを削除します。`,
	RunE: func(cmd *cobra.Command, args []string) (err error) {
		defer derrors.Wrap(&err)

		cfg, err := config.LoadConfig()
		if err != nil {
			return fmt.Errorf("設定の読み込みに失敗しました: %v", err)
		}

		if len(args) == 0 {
			// インタラクティブモード
			return runInteractiveRM(cfg)
		} else {
			// 直接指定モード
			return runDirectRM(cfg, args)
		}
	},
}

func runInteractiveRM(cfg *config.Config) error {
	// チケットを読み込み
	ticketsWithPath, err := loadTicketsFromTmp(cfg.Directory)
	if err != nil {
		return fmt.Errorf("チケットの読み込みに失敗しました: %v", err)
	}

	if len(ticketsWithPath) == 0 {
		fmt.Println("削除可能なチケットが見つかりません")
		return nil
	}

	tty, err := tty.Open()
	if err != nil {
		return err
	}
	defer tty.Close()

	// Bubble Teaアプリを起動
	model, err := newRMModel(ticketsWithPath, cfg.Directory)
	if err != nil {
		return err
	}
	p := tea.NewProgram(model, tea.WithAltScreen(), tea.WithOutput(tty.Output()), tea.WithMouseCellMotion())
	finalModel, err := p.Run()
	if err != nil {
		return err
	}

	rmModel := finalModel.(*rmModel)
	if rmModel.cancelled {
		fmt.Println("削除がキャンセルされました")
		return nil
	}

	selectedTickets := rmModel.SelectedTickets()
	if len(selectedTickets) == 0 {
		fmt.Println("チケットが選択されませんでした")
		return nil
	}

	// 削除実行
	return ui.WithSpinner("チケットを削除中...", func() error {
		for _, item := range selectedTickets {
			if err := deleteTicketWithPath(item); err != nil {
				return fmt.Errorf("チケット %s の削除に失敗しました: %v", item.ticket.Key, err)
			}
		}
		return nil
	})
}

func runDirectRM(cfg *config.Config, ticketKeys []string) error {
	// 指定されたチケットを読み込み
	var ticketItems []rmTicketItem
	for _, key := range ticketKeys {
		filePath := filepath.Join(cfg.Directory, key+".md")
		t, err := ticket.FromFile(filePath)
		if err != nil {
			return fmt.Errorf("チケット %s が見つかりません: %v", key, err)
		}
		// 未pushファイルの場合はキーを「DRAFT」として表示
		displayKey := t.Key
		if !isValidJIRAKey(t.Key) {
			displayKey = "DRAFT"
		}

		ticketItems = append(ticketItems, rmTicketItem{
			key:      displayKey,
			title:    t.Title,
			content:  t.Body,
			ticket:   t,
			filePath: filePath,
		})
	}

	// 削除実行
	return ui.WithSpinner("チケットを削除中...", func() error {
		for _, item := range ticketItems {
			if err := deleteTicketWithPath(item); err != nil {
				return fmt.Errorf("チケット %s の削除に失敗しました: %v", item.key, err)
			}
		}
		return nil
	})
}

func deleteTicket(ticketDir string, t *ticket.Ticket) error {
	originalPath := filepath.Join(ticketDir, t.Key+".md")

	// チケットがJIRAキーを持つかどうかをチェック
	if isValidJIRAKey(t.Key) {
		// JIRAキー付きチケットの場合：ドットプレフィックスでマーク
		deletedPath := filepath.Join(ticketDir, "."+t.Key+".md")
		return os.Rename(originalPath, deletedPath)
	} else {
		// 一時ファイルの場合：物理削除
		return os.Remove(originalPath)
	}
}

func deleteTicketWithPath(item rmTicketItem) error {
	// チケットがJIRAキーを持つかどうかをチェック
	if isValidJIRAKey(item.ticket.Key) {
		// JIRAキー付きチケットの場合：ドットプレフィックスでマーク
		dir := filepath.Dir(item.filePath)
		deletedPath := filepath.Join(dir, "."+item.ticket.Key+".md")
		return os.Rename(item.filePath, deletedPath)
	} else {
		// 一時ファイルの場合：実際のファイルパスを使って物理削除
		return os.Remove(item.filePath)
	}
}

func isValidJIRAKey(key string) bool {
	// JIRAキーの形式をチェック (例: PRJ-123)
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

type ticketWithPath struct {
	ticket   *ticket.Ticket
	filePath string
}

func loadTicketsFromTmp(ticketDir string) ([]ticketWithPath, error) {
	var tickets []ticketWithPath

	err := filepath.WalkDir(ticketDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(path, ".md") {
			// ドットで始まるファイル（既に削除マークされたもの）はスキップ
			filename := filepath.Base(path)
			if strings.HasPrefix(filename, ".") {
				return nil
			}

			t, err := ticket.FromFile(path)
			if err != nil {
				// エラーは無視してスキップ
				return nil
			}
			// 有効なチケット（keyまたはtitleが存在）のみを追加
			if t.Key != "" || t.Title != "" {
				tickets = append(tickets, ticketWithPath{
					ticket:   t,
					filePath: path,
				})
			}
		}
		return nil
	})

	return tickets, err
}

var (
	rmTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("205"))

	rmSearchStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("240")).
			Foreground(lipgloss.Color("230")).
			Padding(0, 1)

	rmSelectedStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("57")).
			Foreground(lipgloss.Color("230"))

	rmBorderStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("63"))

	rmHelpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241"))
)

// rmModel はインタラクティブな削除UI用のモデル
type rmModel struct {
	input         textinput.Model
	mdRenderer    *glamour.TermRenderer
	tickets       []rmTicketItem
	filteredItems []rmTicketItem
	searchQuery   string
	cursor        int
	width         int
	height        int
	ticketDir     string
	selectedMap   map[int]bool // 選択状態を追跡
	cancelled     bool
}

type rmTicketItem struct {
	key      string
	title    string
	content  string
	ticket   *ticket.Ticket
	filePath string
}

func newRMModel(ticketsWithPath []ticketWithPath, ticketDir string) (_ *rmModel, err error) {
	defer derrors.Wrap(&err)
	input := textinput.New()
	input.Focus()

	mdRenderer, err := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithEmoji(),
	)
	if err != nil {
		return nil, err
	}

	// ソート: 新規ファイル（JIRAキーなし）を最初に、その後はupdated_atの降順
	sort.Slice(ticketsWithPath, func(i, j int) bool {
		ticketI := ticketsWithPath[i].ticket
		ticketJ := ticketsWithPath[j].ticket

		// 新規ファイル（JIRAキーが無効）かどうかをチェック
		isNewI := !isValidJIRAKey(ticketI.Key)
		isNewJ := !isValidJIRAKey(ticketJ.Key)

		// 新規ファイルを優先
		if isNewI && !isNewJ {
			return true
		}
		if !isNewI && isNewJ {
			return false
		}

		// 両方とも新規ファイルまたは両方とも既存ファイルの場合はupdated_atで比較
		return ticketI.UpdatedAt.After(ticketJ.UpdatedAt)
	})

	var items []rmTicketItem
	for _, tp := range ticketsWithPath {
		// 空のチケット（keyもtitleも空）をスキップ
		if tp.ticket.Key == "" && tp.ticket.Title == "" {
			continue
		}

		// 未pushファイルの場合はキーを「DRAFT」として表示
		displayKey := tp.ticket.Key
		if !isValidJIRAKey(tp.ticket.Key) {
			displayKey = "DRAFT"
		}

		items = append(items, rmTicketItem{
			key:      displayKey,
			title:    tp.ticket.Title,
			content:  tp.ticket.Body,
			ticket:   tp.ticket,
			filePath: tp.filePath,
		})
	}

	model := &rmModel{
		input:         input,
		mdRenderer:    mdRenderer,
		tickets:       items,
		filteredItems: items,
		searchQuery:   "",
		cursor:        0,
		ticketDir:     ticketDir,
		selectedMap:   make(map[int]bool),
	}

	// 初期状態で最初のファイルを確実に選択
	if len(items) > 0 {
		model.cursor = 0
	}

	return model, nil
}

func (m *rmModel) Init() tea.Cmd {
	return tea.ClearScreen
}

func (m *rmModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			m.cancelled = true
			return m, tea.Quit

		case "enter":
			return m, tea.Quit

		case "tab":
			// タブで選択/非選択を切り替え
			if len(m.filteredItems) > 0 && m.cursor < len(m.filteredItems) {
				// 現在の項目のインデックスを取得
				currentItem := m.filteredItems[m.cursor]
				for i, item := range m.tickets {
					if item.key == currentItem.key {
						m.selectedMap[i] = !m.selectedMap[i]
						break
					}
				}
			}

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

		case "ctrl+k", "ctrl+u":
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

		case " ":
			// スペースを検索文字として追加
			m.searchQuery += " "
			m.filterItems()
			m.cursor = 0

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
				if len(msg.String()) == 1 && msg.String() != "esc" && msg.String() != "tab" {
					m.searchQuery += msg.String()
					m.filterItems()
					m.cursor = 0
				}
			}
		}
	}

	cmds := make([]tea.Cmd, 0)
	input, cmd := m.input.Update(msg)
	m.input = input
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m *rmModel) filterItems() {
	if m.searchQuery == "" {
		m.filteredItems = m.tickets
		// 初期状態では最初のファイルを選択
		if len(m.filteredItems) > 0 && m.cursor >= len(m.filteredItems) {
			m.cursor = 0
		}
		return
	}

	query := strings.ToLower(m.searchQuery)
	var filtered []rmTicketItem
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

func (m *rmModel) View() string {
	// 最小限の表示を保証
	if m.width == 0 {
		m.width = 80
	}
	if m.height == 0 {
		m.height = 24
	}

	// ヘッダー部分
	selectedCount := 0
	for _, selected := range m.selectedMap {
		if selected {
			selectedCount++
		}
	}

	searchLine := fmt.Sprintf("検索: %s", m.searchQuery)
	if selectedCount > 0 {
		searchLine += fmt.Sprintf(" (選択中: %d)", selectedCount)
	}

	helpLine := rmHelpStyle.Render("Tab: 選択/解除  Enter: 削除実行  Esc: キャンセル")
	header := lipgloss.JoinVertical(lipgloss.Left, searchLine, helpLine)

	if len(m.filteredItems) == 0 {
		emptyMsg := lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			Render("チケットが見つかりません")
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
	leftPaneStyled := rmBorderStyle.
		Width(leftWidth - 2).
		Height(availableHeight - 2).
		Render(leftPane)

	// 中央ペイン（チケット内容）
	centerPane := lipgloss.NewStyle().
		MaxHeight(availableHeight - 2).
		Render(
			m.renderCenterPane(centerWidth-2, availableHeight-2),
		)
	centerPaneStyled := rmBorderStyle.
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
	rightPaneStyled := rmBorderStyle.
		Width(rightWidth - 2).
		Height(availableHeight - 2).
		Render(rightPane)

	// 3つのペインを横に並べる
	body := lipgloss.JoinHorizontal(lipgloss.Top, leftPaneStyled, centerPaneStyled, rightPaneStyled)

	return lipgloss.JoinVertical(lipgloss.Left, header, body)
}

func (m *rmModel) renderLeftPane(width, height int) string {
	var items []string

	start := 0
	if m.cursor >= height {
		start = m.cursor - height + 1
	}

	for i := start; i < start+height && i < len(m.filteredItems); i++ {
		item := m.filteredItems[i]

		// この項目が選択されているかチェック
		selected := false
		for j, ticketItem := range m.tickets {
			if ticketItem.key == item.key && m.selectedMap[j] {
				selected = true
				break
			}
		}

		// チェックボックス表示
		checkbox := "[ ]"
		if selected {
			checkbox = "[✓]"
		}

		// キーを固定幅で左詰めパディング（DRAFTやJIRAキーに対応）
		keyPadded := fmt.Sprintf("%-8s", item.key)
		line := fmt.Sprintf("%s %s", checkbox, keyPadded)

		// タイトルがある場合は表示
		if item.title != "" {
			line = fmt.Sprintf("%s %s", line, item.title)
		}

		// 幅に合わせてトリミング
		line = ansi.TruncateWc(line, width, "…")

		if i == m.cursor {
			line = rmSelectedStyle.Width(width).Render(line)
		} else {
			line = lipgloss.NewStyle().Width(width).Render(line)
		}

		items = append(items, line)
	}

	return strings.Join(items, "\n")
}

func (m *rmModel) renderCenterPane(width, height int) string {
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

func (m *rmModel) renderRightPane(width, height int) string {
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

	// 選択されたチケットのticketオブジェクトを取得
	var selectedTicket *ticket.Ticket = m.filteredItems[m.cursor].ticket

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

func (m *rmModel) SelectedTickets() []rmTicketItem {
	var selected []rmTicketItem
	for i, item := range m.tickets {
		if m.selectedMap[i] {
			selected = append(selected, item)
		}
	}
	return selected
}

func init() {
	rootCmd.AddCommand(rmCmd)
}
