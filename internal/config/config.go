package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

// Config は設定ファイルの構造体です
type Config struct {
	AuthType string `mapstructure:"auth_type"`
	Login    string `mapstructure:"login"`
	Server   string `mapstructure:"server"`
	Project  struct {
		Key  string `mapstructure:"key"`
		Type string `mapstructure:"type"`
	} `mapstructure:"project"`
	Board struct {
		ID   int    `mapstructure:"id"`
		Name string `mapstructure:"name"`
		Type string `mapstructure:"type"`
	} `mapstructure:"board"`
	Epic struct {
		Name string `mapstructure:"name"`
		Link string `mapstructure:"link"`
	} `mapstructure:"epic"`
	Issue struct {
		Fields struct {
			Custom []struct {
				Name   string `mapstructure:"name"`
				Key    string `mapstructure:"key"`
				Schema struct {
					Datatype string `mapstructure:"datatype"`
					Items    string `mapstructure:"items"`
				} `mapstructure:"schema"`
			} `mapstructure:"custom"`
		} `mapstructure:"fields"`
		Types []struct {
			ID      string `mapstructure:"id"`
			Name    string `mapstructure:"name"`
			Handle  string `mapstructure:"handle"`
			Subtask bool   `mapstructure:"subtask"`
		} `mapstructure:"types"`
	} `mapstructure:"issue"`
	JQL      string `mapstructure:"jql"`
	Timezone string `mapstructure:"timezone"`
}

// LoadConfig は設定ファイルを読み込みます
func LoadConfig() (*Config, error) {
	// 設定ファイルのパス
	configDir := filepath.Join(os.Getenv("HOME"), ".config", "gojira")
	configFile := filepath.Join(configDir, "config.yml")

	// 設定ファイルが存在するか確認
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		return nil, fmt.Errorf("設定ファイルが見つかりません: %s", configFile)
	}

	// Viperの設定
	viper.SetConfigFile(configFile)
	viper.SetConfigType("yaml")

	// 設定ファイルの読み込み
	if err := viper.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("設定ファイルの読み込みに失敗しました: %v", err)
	}

	// 設定を構造体にマッピング
	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("設定ファイルのパースに失敗しました: %v", err)
	}

	return &config, nil
}

// EnsureCacheDir はキャッシュディレクトリを確保します
func EnsureCacheDir() (string, error) {
	cacheDir := filepath.Join(os.Getenv("HOME"), ".cache", "gojira")
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return "", fmt.Errorf("キャッシュディレクトリの作成に失敗しました: %v", err)
	}
	return cacheDir, nil
}
