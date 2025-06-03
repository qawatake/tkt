package utils

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// PromptForConfirmation はユーザにyesまたはnoの確認を求めます
func PromptForConfirmation(message string) bool {
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Printf("%s [y/N]: ", message)
		response, err := reader.ReadString('\n')
		if err != nil {
			return false
		}

		response = strings.ToLower(strings.TrimSpace(response))
		if response == "y" || response == "yes" {
			return true
		} else if response == "n" || response == "no" || response == "" {
			return false
		}

		fmt.Println("無効な入力です。'y'または'n'を入力してください。")
	}
}

// PromptForChoice はユーザに複数の選択肢から選択を求めます
func PromptForChoice(message string, choices []string) (string, bool) {
	reader := bufio.NewReader(os.Stdin)

	fmt.Printf("%s\n", message)
	for i, choice := range choices {
		fmt.Printf("%d. %s\n", i+1, choice)
	}

	for {
		fmt.Print("選択肢を入力してください (番号または文字): ")
		response, err := reader.ReadString('\n')
		if err != nil {
			return "", false
		}

		response = strings.ToLower(strings.TrimSpace(response))

		// 文字列での選択をチェック
		for _, choice := range choices {
			if strings.ToLower(choice) == response {
				return choice, true
			}
		}

		// 番号での選択をチェック
		for i, choice := range choices {
			if fmt.Sprintf("%d", i+1) == response {
				return choice, true
			}
		}

		fmt.Printf("無効な入力です。%sのいずれかを入力してください。\n", strings.Join(choices, ", "))
	}
}
