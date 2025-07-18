package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/qawatake/tkt/internal/config"
	"github.com/qawatake/tkt/internal/jira"
	"github.com/qawatake/tkt/internal/ticket"
	"github.com/qawatake/tkt/internal/ui"
	"github.com/spf13/cobra"
)

var createCmd = &cobra.Command{
	Use:     "create",
	Aliases: []string{"c"},
	Short:   "新しいJIRAチケットをインタラクティブに作成します",
	Long: `新しいJIRAチケットをインタラクティブに作成します。
タイトル、タイプを入力し、vimエディタでボディを編集できます。`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runCreate()
	},
}

func init() {
	rootCmd.AddCommand(createCmd)
}

func runCreate() error {
	// 設定ファイルを読み込み
	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("設定ファイルの読み込みに失敗しました: %v\n'tkt init' コマンドで設定ファイルを作成してください", err)
	}

	fmt.Println("🎫 新しいJIRAチケット作成")
	fmt.Println("========================")

	var title, selectedType string

	// 1. タイトルとチケットタイプを入力
	availableTypes := cfg.Issue.Types
	if len(availableTypes) == 0 {
		return fmt.Errorf("プロジェクト '%s' に対応するチケットタイプが見つかりません", cfg.Project.Key)
	}

	// チケットタイプの選択肢を準備
	typeOptions := make([]huh.Option[string], len(availableTypes))
	for i, issueType := range availableTypes {
		typeOptions[i] = huh.NewOption(issueType.Name, issueType.Name)
	}

	basicForm := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("チケットタイトル").
				Description("作成するチケットのタイトル (例: ユーザー登録機能を追加)").
				Value(&title).
				Validate(func(s string) error {
					if s == "" {
						return fmt.Errorf("チケットタイトルは必須です")
					}
					return nil
				}),

			huh.NewSelect[string]().
				Title("チケットタイプ").
				Description("作成するチケットの種類を選択").
				Options(typeOptions...).
				Value(&selectedType).
				Validate(func(s string) error {
					if s == "" {
						return fmt.Errorf("チケットタイプの選択は必須です")
					}
					return nil
				}),
		),
	).WithTheme(huh.ThemeBase())

	err = basicForm.Run()
	if err != nil {
		return fmt.Errorf("基本情報の入力がキャンセルされました: %v", err)
	}

	// 3. スプリント選択
	var selectedSprintName string

	if cfg.Board.ID != 0 {
		// JIRAクライアントを作成
		jiraClient, err := jira.NewClient(cfg)
		if err != nil {
			return fmt.Errorf("JIRAクライアントの作成に失敗しました: %v", err)
		}

		// アクティブと未来のスプリントを取得
		sprints, err := ui.WithSpinnerValue("スプリント情報を取得中...", func() ([]jira.Sprint, error) {
			return jiraClient.GetActiveAndFutureSprints(cfg.Board.ID)
		})
		if err != nil {
			fmt.Printf("⚠️  スプリント情報の取得に失敗しました: %v\n", err)
			fmt.Println("スプリントを選択せずに作成を続行します...")
		} else if len(sprints) > 0 {
			// スプリントを状態でソート（active -> future）
			sort.Slice(sprints, func(i, j int) bool {
				stateOrder := map[string]int{"active": 0, "future": 1}
				return stateOrder[sprints[i].State] < stateOrder[sprints[j].State]
			})

			// スプリント選択オプションを準備
			sprintSelectorOptions := make([]ui.SelectorOption, len(sprints)+1)

			// "スプリントに追加しない"オプションを先頭に追加
			sprintSelectorOptions[0] = ui.SelectorOption{
				Title:       "スプリントに追加しない",
				Description: "スプリントを指定せずにチケットを作成",
				Value:       "",
			}

			for i, sprint := range sprints {
				statusEmoji := ""
				switch sprint.State {
				case "active":
					statusEmoji = "🟢 "
				case "future":
					statusEmoji = "🔵 "
				}

				sprintSelectorOptions[i+1] = ui.SelectorOption{
					Title:       fmt.Sprintf("%s%s (%s)", statusEmoji, sprint.Name, sprint.State),
					Description: fmt.Sprintf("ID: %d | 開始: %s | 終了: %s", sprint.ID, sprint.StartDate, sprint.EndDate),
					Value:       sprint.Name,
				}
			}

			selectedSprintValue, err := ui.Select("🏃 スプリントを選択してください:", sprintSelectorOptions)
			if err != nil {
				fmt.Printf("⚠️  スプリント選択がキャンセルされました: %v\n", err)
				fmt.Println("スプリントを選択せずに作成を続行します...")
			} else {
				selectedSprintName = selectedSprintValue.(string)
			}
		}
	} else {
		fmt.Println("\n⚠️  ボード設定が見つかりません。スプリント選択はスキップします。")
	}

	// 4. ボディをvimエディタで入力
	fmt.Println("\n📝 ボディを編集します (vimエディタが開きます)...")
	body, err := openEditor()
	if err != nil {
		if strings.Contains(err.Error(), "保存せずに終了") {
			fmt.Println("⚠️ エディタが保存せずに終了されたため、チケット作成をキャンセルします。")
			return nil
		}
		return fmt.Errorf("エディタの起動に失敗しました: %v", err)
	}

	// 5. ローカルチケットを作成 (keyは空文字列、リモートが採番)
	newTicket := &ticket.Ticket{
		Key:        "", // リモートが採番するため空文字列
		Title:      title,
		Type:       selectedType,
		Body:       body,
		SprintName: selectedSprintName,
	}

	// 6. ローカルファイルとして保存
	fmt.Println("\n💾 ローカルファイルを保存中...")
	filePath, err := ui.WithSpinnerValue("ローカルファイルを保存中...", func() (string, error) {
		return newTicket.SaveToFile(cfg.Directory)
	})
	if err != nil {
		return fmt.Errorf("ローカルファイルの保存に失敗しました: %v", err)
	}

	fmt.Println("\n✅ ローカルチケットが作成されました！")
	fmt.Printf("   タイトル: %s\n", newTicket.Title)
	fmt.Printf("   タイプ: %s\n", newTicket.Type)
	if selectedSprintName != "" {
		fmt.Printf("   スプリント: %s\n", selectedSprintName)
	}
	fmt.Printf("   ファイル: %s\n", filePath)
	fmt.Printf("   次のステップ: 'tkt push' でJIRAに同期してキーを取得\n")

	return nil
}

// openEditor はvimエディタを開いてユーザーに入力させます
func openEditor() (string, error) {
	// 一時ファイルを作成
	tmpFile, err := os.CreateTemp("", "tkt-create-*.md")
	if err != nil {
		return "", fmt.Errorf("一時ファイルの作成に失敗しました: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	// ファイルの初期状態を記録
	initialStat, err := os.Stat(tmpFile.Name())
	if err != nil {
		return "", fmt.Errorf("ファイル情報の取得に失敗しました: %v", err)
	}
	initialModTime := initialStat.ModTime()
	initialSize := initialStat.Size()

	tmpFile.Close()

	// vimエディタを起動 (insertモードで開始)
	cmd := exec.Command("vim", "+startinsert", tmpFile.Name())
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("vimエディタの実行に失敗しました: %v", err)
	}

	// ファイルの変更を確認
	finalStat, err := os.Stat(tmpFile.Name())
	if err != nil {
		return "", fmt.Errorf("ファイル情報の取得に失敗しました: %v", err)
	}

	// ファイルが変更されていない場合（サイズも変更時刻も同じ）は保存されていないと判断
	if finalStat.ModTime().Equal(initialModTime) && finalStat.Size() == initialSize {
		return "", fmt.Errorf("エディタが保存せずに終了されました")
	}

	// ファイルの内容を読み取り
	content, err := os.ReadFile(tmpFile.Name())
	if err != nil {
		return "", fmt.Errorf("ファイルの読み取りに失敗しました: %v", err)
	}

	body := strings.TrimSpace(string(content))

	return body, nil
}
