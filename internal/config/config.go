package config

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

// Config は設定ファイルの構造体です
type Config struct {
	AuthType string `mapstructure:"auth_type" yaml:"auth_type"`
	Login    string `mapstructure:"login" yaml:"login"`
	Server   string `mapstructure:"server" yaml:"server"`
	Project  struct {
		Key  string `mapstructure:"key" yaml:"key"`
		Type string `mapstructure:"type" yaml:"type"`
	} `mapstructure:"project" yaml:"project"`
	Board struct {
		ID   int    `mapstructure:"id" yaml:"id"`
		Name string `mapstructure:"name" yaml:"name"`
		Type string `mapstructure:"type" yaml:"type"`
	} `mapstructure:"board" yaml:"board"`
	Epic struct {
		Name string `mapstructure:"name" yaml:"name"`
		Link string `mapstructure:"link" yaml:"link"`
	} `mapstructure:"epic" yaml:"epic"`
	Issue struct {
		Fields struct {
			Custom []struct {
				Name   string `mapstructure:"name" yaml:"name"`
				Key    string `mapstructure:"key" yaml:"key"`
				Schema struct {
					Datatype string `mapstructure:"datatype" yaml:"datatype"`
					Items    string `mapstructure:"items" yaml:"items"`
				} `mapstructure:"schema" yaml:"schema"`
			} `mapstructure:"custom" yaml:"custom"`
		} `mapstructure:"fields" yaml:"fields"`
		Types []struct {
			ID      string `mapstructure:"id" yaml:"id"`
			Name    string `mapstructure:"name" yaml:"name"`
			Handle  string `mapstructure:"handle" yaml:"handle"`
			Subtask bool   `mapstructure:"subtask" yaml:"subtask"`
		} `mapstructure:"types" yaml:"types"`
	} `mapstructure:"issue" yaml:"issue"`
	JQL       string `mapstructure:"jql" yaml:"jql"`
	Timezone  string `mapstructure:"timezone" yaml:"timezone"`
	Directory string `mapstructure:"directory" yaml:"directory"`
}

// LoadConfig は設定ファイルを読み込みます
func LoadConfig() (*Config, error) {
	// 設定ファイルのパス (カレントディレクトリのticket.yml)
	configFile := "ticket.yml"

	// 設定ファイルが存在するか確認
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		return nil, fmt.Errorf("設定ファイルが見つかりません: %s\n'tkt init'コマンドで設定ファイルを作成してください", configFile)
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
	config, err := LoadConfig()
	if err != nil {
		return "", fmt.Errorf("設定の読み込みに失敗しました: %v", err)
	}

	workDir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("作業ディレクトリの取得に失敗しました: %v", err)
	}

	cacheDir := getCacheDir(config, workDir)

	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return "", fmt.Errorf("キャッシュディレクトリの作成に失敗しました: %v", err)
	}
	return cacheDir, nil
}

// getCacheDir はプロジェクト固有のキャッシュディレクトリパスを生成します
func getCacheDir(config *Config, workDir string) string {
	// ハッシュ値を生成するための文字列を作成
	hashInput := fmt.Sprintf("%s|%s|%s", workDir, config.Server, config.JQL)

	// SHA256ハッシュを計算
	hash := sha256.Sum256([]byte(hashInput))
	hashStr := fmt.Sprintf("%x", hash)[:16] // 最初の16文字を使用

	// キャッシュディレクトリパスを生成
	baseCacheDir := filepath.Join(os.Getenv("HOME"), ".cache", "tkt")
	cacheDir := filepath.Join(baseCacheDir, hashStr)

	return cacheDir
}
