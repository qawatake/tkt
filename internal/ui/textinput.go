package ui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

type textInputModel struct {
	textInput   textinput.Model
	prompt      string
	placeholder string
	required    bool
	err         error
	done        bool
	value       string
}

func (m textInputModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m textInputModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			m.err = fmt.Errorf("入力がキャンセルされました")
			m.done = true
			return m, tea.Quit
		case "enter":
			value := m.textInput.Value()
			if m.required && value == "" {
				// エラーメッセージを表示するが、入力を続行
				return m, nil
			}
			m.value = value
			m.done = true
			return m, tea.Quit
		}
	}

	m.textInput, cmd = m.textInput.Update(msg)
	return m, cmd
}

func (m textInputModel) View() string {
	var view string
	if m.placeholder != "" {
		view = fmt.Sprintf("%s\n例: %s\n%s", m.prompt, m.placeholder, m.textInput.View())
	} else {
		view = fmt.Sprintf("%s\n%s", m.prompt, m.textInput.View())
	}
	
	if m.required && m.textInput.Value() == "" && m.textInput.Focused() {
		view += "\n\n⚠️  この項目は必須です"
	}
	
	return view
}

// PromptForText はbubbletea textinputを使用してテキスト入力を取得します
func PromptForText(prompt string, placeholder string, required bool) (string, error) {
	ti := textinput.New()
	ti.Focus()
	ti.CharLimit = 200
	ti.Width = 50

	m := textInputModel{
		textInput:   ti,
		prompt:      prompt,
		placeholder: placeholder,
		required:    required,
	}

	p := tea.NewProgram(m)
	finalModel, err := p.Run()
	if err != nil {
		return "", err
	}

	result := finalModel.(textInputModel)
	if result.err != nil {
		return "", result.err
	}

	return result.value, nil
}
