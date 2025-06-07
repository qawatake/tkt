package ui

import (
	"time"

	"github.com/briandowns/spinner"
)

// Spinner は読み込み中のスピナーを表示するためのwrapperです
type Spinner struct {
	spinner *spinner.Spinner
}

// New は新しいスピナーを作成します
func NewSpinner() *Spinner {
	s := spinner.New(spinner.CharSets[14], 100*time.Millisecond)
	return &Spinner{spinner: s}
}

// Start はスピナーを開始します
func (s *Spinner) Start(message string) {
	s.spinner.Suffix = " " + message
	s.spinner.Start()
}

// Stop はスピナーを停止します
func (s *Spinner) Stop() {
	s.spinner.Stop()
}

// Update はスピナーのメッセージを更新します
func (s *Spinner) Update(message string) {
	s.spinner.Suffix = " " + message
}

// WithSpinner は指定された処理中にスピナーを表示します
func WithSpinner(message string, fn func() error) error {
	s := NewSpinner()
	s.Start(message)
	defer s.Stop()
	return fn()
}

// WithSpinnerValue は指定された処理中にスピナーを表示し、値を返します
func WithSpinnerValue[T any](message string, fn func() (T, error)) (T, error) {
	s := NewSpinner()
	s.Start(message)
	defer s.Stop()
	return fn()
}

// FetchWithSpinner はJIRA APIの取得処理でよく使われるパターンのヘルパー関数です
func FetchWithSpinner[T any](resource string, fn func() (T, error)) (T, error) {
	return WithSpinnerValue(resource+"を取得中...", fn)
}
