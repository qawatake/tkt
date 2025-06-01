package config

import (
	"time"
)

// ParseTime は時間文字列をtime.Time型に変換します
func ParseTime(timeStr string) time.Time {
	t, _ := time.Parse(time.RFC3339, timeStr)
	return t
}
