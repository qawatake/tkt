package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetCacheDir(t *testing.T) {
	tests := []struct {
		name            string
		config1         *Config
		config2         *Config
		workDir1        string
		workDir2        string
		expectDifferent bool
	}{
		{
			name: "different JQL should generate different cache dirs",
			config1: &Config{
				Server: "https://company.atlassian.net",
				JQL:    "project = TEST",
			},
			config2: &Config{
				Server: "https://company.atlassian.net",
				JQL:    "project = PROD",
			},
			workDir1:        "/tmp/project1",
			workDir2:        "/tmp/project1",
			expectDifferent: true,
		},
		{
			name: "different server should generate different cache dirs",
			config1: &Config{
				Server: "https://company1.atlassian.net",
				JQL:    "project = TEST",
			},
			config2: &Config{
				Server: "https://company2.atlassian.net",
				JQL:    "project = TEST",
			},
			workDir1:        "/tmp/project1",
			workDir2:        "/tmp/project1",
			expectDifferent: true,
		},
		{
			name: "different work directory should generate different cache dirs",
			config1: &Config{
				Server: "https://company.atlassian.net",
				JQL:    "project = TEST",
			},
			config2: &Config{
				Server: "https://company.atlassian.net",
				JQL:    "project = TEST",
			},
			workDir1:        "/tmp/project1",
			workDir2:        "/tmp/project2",
			expectDifferent: true,
		},
		{
			name: "same config should generate same cache dir",
			config1: &Config{
				Server: "https://company.atlassian.net",
				JQL:    "project = TEST",
			},
			config2: &Config{
				Server: "https://company.atlassian.net",
				JQL:    "project = TEST",
			},
			workDir1:        "/tmp/project1",
			workDir2:        "/tmp/project1",
			expectDifferent: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 最初の設定でキャッシュディレクトリを取得
			cacheDir1 := getCacheDir(tt.config1, tt.workDir1)

			// 2番目の設定でキャッシュディレクトリを取得
			cacheDir2 := getCacheDir(tt.config2, tt.workDir2)

			// キャッシュディレクトリが期待通りかチェック
			if tt.expectDifferent {
				assert.NotEqual(t, cacheDir1, cacheDir2, "Expected different cache directories")
			} else {
				assert.Equal(t, cacheDir1, cacheDir2, "Expected same cache directories")
			}

			// キャッシュディレクトリが正しい形式かチェック
			homeDir := os.Getenv("HOME")
			expectedPrefix := filepath.Join(homeDir, ".cache", "tkt")
			assert.True(t, strings.HasPrefix(cacheDir1, expectedPrefix), "Cache dir should be under ~/.cache/tkt")
			assert.True(t, strings.HasPrefix(cacheDir2, expectedPrefix), "Cache dir should be under ~/.cache/tkt")
		})
	}
}
