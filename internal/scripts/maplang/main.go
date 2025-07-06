package main

import (
	_ "embed"
	"fmt"
	"log"
	"os"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

type Language struct {
	Type    string   `yaml:"type"`
	Color   string   `yaml:"color"`
	Aliases []string `yaml:"aliases"`
}

type Languages map[string]Language

//go:embed languages.yml
var languagesData []byte

//go:embed normalized.txt
var normalizedData []byte

func main() {
	var languages Languages
	if err := yaml.Unmarshal(languagesData, &languages); err != nil {
		log.Fatalf("Failed to parse languages.yml: %v", err)
	}

	targetLanguages := extractTargetLanguages(string(normalizedData))

	// Debug: output target languages to stderr
	fmt.Fprintf(os.Stderr, "Found %d target languages:\n", len(targetLanguages))
	for langName, targetKey := range targetLanguages {
		fmt.Fprintf(os.Stderr, "  %s -> %s\n", langName, targetKey)
	}
	fmt.Fprintf(os.Stderr, "\n")

	mapping := generateLanguageMapping(languages, targetLanguages)

	fmt.Println("// Auto-generated language mapping")
	fmt.Println("var languageMap = map[string]string{")

	var keys []string
	for k := range mapping {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, key := range keys {
		fmt.Printf("\t%q: %q,\n", key, mapping[key])
	}

	fmt.Println("}")
}

func extractTargetLanguages(lllContent string) map[string]string {
	targetLanguages := make(map[string]string)

	re := regexp.MustCompile(`^\s*\d*→?\s*([^(]+)\s*\(([^)]+)\)`)

	lines := strings.Split(lllContent, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if matches := re.FindStringSubmatch(line); len(matches) == 3 {
			languageName := strings.TrimSpace(matches[1])
			languageKey := strings.TrimSpace(matches[2])
			targetLanguages[languageName] = languageKey
		}
	}

	return targetLanguages
}

func generateLanguageMapping(languages Languages, targetLanguages map[string]string) map[string]string {
	mapping := make(map[string]string)

	for langName, targetKey := range targetLanguages {
		normalizedKey := strings.ToLower(targetKey)

		// Add the normalized key itself (e.g., "go" -> "go")
		mapping[normalizedKey] = normalizedKey

		// Try exact match first
		if langDef, exists := languages[langName]; exists {
			for _, alias := range langDef.Aliases {
				normalizedAlias := strings.ToLower(strings.ReplaceAll(alias, " ", "_"))
				mapping[normalizedAlias] = normalizedKey
			}
			continue
		}

		// Try case-insensitive match
		for yamlLangName, langDef := range languages {
			if strings.EqualFold(yamlLangName, langName) {
				for _, alias := range langDef.Aliases {
					normalizedAlias := strings.ToLower(strings.ReplaceAll(alias, " ", "_"))
					mapping[normalizedAlias] = normalizedKey
				}
				break
			}
		}
	}

	return mapping
}
