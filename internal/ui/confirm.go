package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

type confirmModel struct {
	textInput textinput.Model
	prompt    string
	err       error
	done      bool
	result    bool
}

func (m confirmModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m confirmModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			m.err = fmt.Errorf("入力がキャンセルされました")
			m.done = true
			return m, tea.Quit
		case "enter":
			value := strings.ToLower(strings.TrimSpace(m.textInput.Value()))
			m.result = value == "y" || value == "yes"
			m.done = true
			return m, tea.Quit
		}
	}

	m.textInput, cmd = m.textInput.Update(msg)
	return m, cmd
}

func (m confirmModel) View() string {
	return fmt.Sprintf("%s\n例: y または yes\n%s\n\n💡 y/yes で継続、その他で中止", m.prompt, m.textInput.View())
}

// PromptForConfirmation はbubbletea textinputを使用してy/n確認を取得します
func PromptForConfirmation(prompt string) (bool, error) {
	ti := textinput.New()
	ti.Focus()
	ti.CharLimit = 10
	ti.Width = 20

	m := confirmModel{
		textInput: ti,
		prompt:    prompt,
	}

	p := tea.NewProgram(m)
	finalModel, err := p.Run()
	if err != nil {
		return false, err
	}

	result := finalModel.(confirmModel)
	if result.err != nil {
		return false, result.err
	}

	return result.result, nil
}
