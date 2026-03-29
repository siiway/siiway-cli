package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/SiiWay/siiway-cli/internal/appconfig"
	"gopkg.in/yaml.v3"
)

type projectConfig struct {
	Language  string                              `yaml:"language"`
	Languages map[string]appconfig.LanguageConfig `yaml:"languages"`
}

func loadProjectConfig(path string) (projectConfig, error) {
	cfg := projectConfig{Languages: map[string]appconfig.LanguageConfig{}}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return cfg, fmt.Errorf("failed reading %s: %w", path, err)
	}

	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("failed parsing %s: %w", path, err)
	}

	cfg.Language = strings.ToLower(strings.TrimSpace(cfg.Language))
	cfg.Languages = normalizeLanguages(cfg.Languages)
	return cfg, nil
}

func normalizeLanguages(in map[string]appconfig.LanguageConfig) map[string]appconfig.LanguageConfig {
	out := map[string]appconfig.LanguageConfig{}
	for lang, langCfg := range in {
		normalizedLang := strings.ToLower(strings.TrimSpace(lang))
		if normalizedLang == "" {
			continue
		}

		runMap := map[string]string{}
		for action, command := range langCfg.Run {
			normalizedAction := strings.ToLower(strings.TrimSpace(action))
			normalizedCommand := strings.TrimSpace(command)
			if normalizedAction == "" || normalizedCommand == "" {
				continue
			}
			runMap[normalizedAction] = normalizedCommand
		}

		out[normalizedLang] = appconfig.LanguageConfig{
			Run: runMap,
			Version: appconfig.LanguageVersionConfig{
				Backend: strings.ToLower(strings.TrimSpace(langCfg.Version.Backend)),
				Use:     strings.TrimSpace(langCfg.Version.Use),
			},
		}
	}
	return out
}

func mergeLanguages(global, project map[string]appconfig.LanguageConfig) map[string]appconfig.LanguageConfig {
	merged := map[string]appconfig.LanguageConfig{}

	for lang, langCfg := range normalizeLanguages(global) {
		copiedRun := map[string]string{}
		for action, command := range langCfg.Run {
			copiedRun[action] = command
		}
		merged[lang] = appconfig.LanguageConfig{
			Run:     copiedRun,
			Version: langCfg.Version,
		}
	}

	for lang, langCfg := range normalizeLanguages(project) {
		base := merged[lang]
		if base.Run == nil {
			base.Run = map[string]string{}
		}
		for action, command := range langCfg.Run {
			base.Run[action] = command
		}
		if langCfg.Version.Backend != "" {
			base.Version.Backend = langCfg.Version.Backend
		}
		if langCfg.Version.Use != "" {
			base.Version.Use = langCfg.Version.Use
		}
		merged[lang] = base
	}

	return merged
}

func detectProjectLanguage(cwd string, languages map[string]appconfig.LanguageConfig) (string, error) {
	if _, ok := languages["python"]; ok {
		if fileExists(filepath.Join(cwd, "pyproject.toml")) || fileExists(filepath.Join(cwd, "requirements.txt")) || fileExists(filepath.Join(cwd, "setup.py")) {
			return "python", nil
		}
	}

	if _, ok := languages["node"]; ok {
		if fileExists(filepath.Join(cwd, "package.json")) || fileExists(filepath.Join(cwd, "bun.lockb")) || fileExists(filepath.Join(cwd, "pnpm-lock.yaml")) || fileExists(filepath.Join(cwd, "yarn.lock")) {
			return "node", nil
		}
	}

	if len(languages) == 1 {
		for lang := range languages {
			return lang, nil
		}
	}

	return "", fmt.Errorf("unable to detect project language in %s (configured languages: %s)", cwd, strings.Join(sortedKeysLang(languages), ", "))
}

func resolveLanguage(cwd, preferredLanguage string, languages map[string]appconfig.LanguageConfig) (string, error) {
	preferredLanguage = strings.ToLower(strings.TrimSpace(preferredLanguage))
	if preferredLanguage != "" {
		if _, ok := languages[preferredLanguage]; !ok {
			return "", fmt.Errorf("language %q configured in .siiway.yaml is not defined in languages", preferredLanguage)
		}
		return preferredLanguage, nil
	}

	return detectProjectLanguage(cwd, languages)
}

func splitArgsAndOptions(tokens []string) ([]string, []string) {
	for i, token := range tokens {
		if token == "--" {
			return tokens[:i], tokens[i+1:]
		}
	}
	return tokens, nil
}

func joinShellEscaped(parts []string) string {
	if len(parts) == 0 {
		return ""
	}
	escaped := make([]string, 0, len(parts))
	for _, part := range parts {
		escaped = append(escaped, shellQuote(part))
	}
	return strings.Join(escaped, " ")
}

func shellQuote(s string) string {
	if s == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func sortedKeysLang(m map[string]appconfig.LanguageConfig) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
