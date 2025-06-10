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
