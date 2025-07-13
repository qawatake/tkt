package config

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/qawatake/tkt/internal/derrors"
	"github.com/spf13/viper"
)

// IssueType はJIRAのIssue Type情報を表します
type IssueType struct {
	ID               string `mapstructure:"id" yaml:"id"`
	Description      string `mapstructure:"description" yaml:"description"`
	Name             string `mapstructure:"name" yaml:"name"`
	UntranslatedName string `mapstructure:"untranslated_name" yaml:"untranslated_name"`
	Subtask          bool   `mapstructure:"subtask" yaml:"subtask"`
}

// Config は設定ファイルの構造体です
type Config struct {
	AuthType string `mapstructure:"auth_type" yaml:"auth_type"`
	Login    string `mapstructure:"login" yaml:"login"`
	Server   string `mapstructure:"server" yaml:"server"`
	Project  struct {
		Key  string `mapstructure:"key" yaml:"key"`
		ID   string `mapstructure:"id" yaml:"id"`
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
		// プロジェクトで利用可能なIssue Typeのリスト
		// チケットを作成するときはこの中から選択する必要があります。
		Types []IssueType `mapstructure:"types" yaml:"types"`
	} `mapstructure:"issue" yaml:"issue"`
	JQL       string `mapstructure:"jql" yaml:"jql"`
	Timezone  string `mapstructure:"timezone" yaml:"timezone"`
	Directory string `mapstructure:"directory" yaml:"directory"`
}

// LoadConfig は設定ファイルを読み込みます
func LoadConfig() (*Config, error) {
	// 設定ファイルのパス (カレントディレクトリのtkt.yml)
	configFile := "tkt.yml"

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

// ClearCacheDir はキャッシュディレクトリを削除し、再作成します
func ClearCacheDir() (_ string, err error) {
	defer derrors.Wrap(&err)

	config, err := LoadConfig()
	if err != nil {
		return "", err
	}

	workDir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	cacheDir := getCacheDir(config, workDir)

	// キャッシュディレクトリを削除
	if err := os.RemoveAll(cacheDir); err != nil {
		return "", err
	}

	// 再度ディレクトリを作成
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return "", err
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

// GetLastFetchTime は最終フェッチ時刻を読み込みます
func GetLastFetchTime() (time.Time, error) {
	cacheDir, err := EnsureCacheDir()
	if err != nil {
		return time.Time{}, fmt.Errorf("キャッシュディレクトリの確保に失敗しました: %v", err)
	}

	timestampFile := filepath.Join(cacheDir, "last_fetch.txt")
	data, err := os.ReadFile(timestampFile)
	if err != nil {
		if os.IsNotExist(err) {
			return time.Time{}, nil
		}
		return time.Time{}, fmt.Errorf("最終フェッチ時刻ファイルの読み込みに失敗しました: %v", err)
	}

	timestamp, err := time.Parse(time.RFC3339, string(data))
	if err != nil {
		return time.Time{}, fmt.Errorf("最終フェッチ時刻のパースに失敗しました: %v", err)
	}

	return timestamp, nil
}

// SaveLastFetchTime は最終フェッチ時刻を保存します
func SaveLastFetchTime(timestamp time.Time) error {
	cacheDir, err := EnsureCacheDir()
	if err != nil {
		return fmt.Errorf("キャッシュディレクトリの確保に失敗しました: %v", err)
	}

	timestampFile := filepath.Join(cacheDir, "last_fetch.txt")
	data := timestamp.Format(time.RFC3339)

	err = os.WriteFile(timestampFile, []byte(data), 0644)
	if err != nil {
		return fmt.Errorf("最終フェッチ時刻の保存に失敗しました: %v", err)
	}

	return nil
}
