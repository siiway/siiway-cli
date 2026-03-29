package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
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

// languageRuntimeContext stores resolved runtime data used by language-aware commands.
type languageRuntimeContext struct {
	Cwd       string
	Project   projectConfig
	Languages map[string]appconfig.LanguageConfig
}

// loadLanguageRuntimeContext loads global/project config, merges language settings,
// and returns the effective runtime context for the current working directory.
func loadLanguageRuntimeContext() (languageRuntimeContext, error) {
	cfg, err := appconfig.Load()
	if err != nil {
		return languageRuntimeContext{}, fmt.Errorf("failed loading global config: %w", err)
	}

	cwd, err := os.Getwd()
	if err != nil {
		return languageRuntimeContext{}, fmt.Errorf("failed getting current directory: %w", err)
	}

	projectCfg, err := loadProjectConfig(filepath.Join(cwd, ".siiway.yaml"))
	if err != nil {
		return languageRuntimeContext{}, err
	}

	languages := mergeLanguages(cfg.Languages, projectCfg.Languages)

	return languageRuntimeContext{
		Cwd:       cwd,
		Project:   projectCfg,
		Languages: languages,
	}, nil
}

// parseListInvocation parses shared list flags (-l/--list) and returns remaining args.
func parseListInvocation(commandName string, args []string) (bool, []string, error) {
	listMode := false
	remaining := args

	for len(remaining) > 0 {
		token := strings.TrimSpace(remaining[0])
		switch token {
		case "-l", "--list":
			listMode = true
			remaining = remaining[1:]
		case "--":
			return listMode, remaining, nil
		default:
			if strings.HasPrefix(token, "-") {
				return false, nil, fmt.Errorf("unknown flag for %s: %s", commandName, token)
			}
			return listMode, remaining, nil
		}
	}

	return listMode, remaining, nil
}

// runShellCommand executes a rendered shell command in the provided working directory.
func runShellCommand(ctx context.Context, cwd, rendered string) error {
	execCmd := exec.CommandContext(ctx, "sh", "-c", rendered)
	execCmd.Stdout = os.Stdout
	execCmd.Stderr = os.Stderr
	execCmd.Stdin = os.Stdin
	execCmd.Dir = cwd
	return execCmd.Run()
}

// loadProjectConfig reads .siiway.yaml-like project config and normalizes its language keys.
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

// normalizeLanguages normalizes language names, run action keys, and version config values.
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

// mergeLanguages merges global and project language config, with project values taking precedence.
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

// detectProjectLanguage infers project language from known marker files in the current directory.
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

// resolveLanguage returns explicitly configured language, otherwise falls back to auto detection.
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

// splitArgsAndOptions splits positional args and -- options into two slices.
func splitArgsAndOptions(tokens []string) ([]string, []string) {
	for i, token := range tokens {
		if token == "--" {
			return tokens[:i], tokens[i+1:]
		}
	}
	return tokens, nil
}

// joinShellEscaped safely quotes and joins shell arguments into a single string.
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

// shellQuote wraps one argument in single quotes and escapes embedded quotes.
func shellQuote(s string) string {
	if s == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
}

// fileExists reports whether a regular file exists at path.
func fileExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

// sortedKeys returns sorted keys from a string map.
func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// sortedKeysLang returns sorted language keys from a language config map.
func sortedKeysLang(m map[string]appconfig.LanguageConfig) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
