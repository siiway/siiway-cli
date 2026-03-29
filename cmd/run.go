package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/SiiWay/siiway-cli/internal/appconfig"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

func init() {
	rootCmd.AddCommand(runCmd)
}

var runCmd = &cobra.Command{
	Use:                "run <action> [args...] [-- options...]",
	Short:              "Run project commands from configuration",
	DisableFlagParsing: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		listMode, remainingArgs, err := parseRunInvocation(args)
		if err != nil {
			return err
		}

		cfg, err := appconfig.Load()
		if err != nil {
			return fmt.Errorf("failed loading global config: %w", err)
		}

		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed getting current directory: %w", err)
		}

		projectCfg, err := loadProjectConfig(filepath.Join(cwd, ".siiway.yaml"))
		if err != nil {
			return err
		}

		languages := mergeLanguages(cfg.Languages, projectCfg.Languages)
		if len(languages) == 0 {
			return fmt.Errorf("no languages.run configuration found in global config or .siiway.yaml")
		}

		preferredLanguage := strings.ToLower(strings.TrimSpace(projectCfg.Language))
		if preferredLanguage != "" {
			if _, ok := languages[preferredLanguage]; !ok {
				return fmt.Errorf("language %q configured in .siiway.yaml is not defined in languages", preferredLanguage)
			}
		}

		if listMode {
			if len(remainingArgs) > 0 {
				return fmt.Errorf("--list (-l) does not accept action or arguments")
			}
			printAvailableRunActions(cwd, preferredLanguage, languages)
			return nil
		}
		if len(remainingArgs) == 0 {
			return fmt.Errorf("action is required")
		}

		lang := preferredLanguage
		if lang == "" {
			lang, err = detectProjectLanguage(cwd, languages)
			if err != nil {
				return err
			}
		}

		action := strings.ToLower(strings.TrimSpace(remainingArgs[0]))
		runMap := languages[lang].Run
		commandTemplate, ok := runMap[action]
		if !ok {
			return fmt.Errorf("run action not found for language %q: %s (available: %s)", lang, action, strings.Join(sortedKeys(runMap), ", "))
		}

		arguments, options := splitRunArgsAndOptions(remainingArgs[1:])
		rendered := renderRunCommand(commandTemplate, arguments, options)
		if strings.TrimSpace(rendered) == "" {
			return fmt.Errorf("resolved command is empty for language %q action %q", lang, action)
		}

		execCmd := exec.CommandContext(cmd.Context(), "sh", "-c", rendered)
		execCmd.Stdout = os.Stdout
		execCmd.Stderr = os.Stderr
		execCmd.Stdin = os.Stdin
		execCmd.Dir = cwd
		if err := execCmd.Run(); err != nil {
			return fmt.Errorf("run command failed: %w", err)
		}

		return nil
	},
}

func parseRunInvocation(args []string) (bool, []string, error) {
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
				return false, nil, fmt.Errorf("unknown flag for run: %s", token)
			}
			return listMode, remaining, nil
		}
	}

	return listMode, remaining, nil
}

func printAvailableRunActions(cwd, preferredLanguage string, languages map[string]appconfig.LanguageConfig) {
	if preferredLanguage != "" {
		runMap := languages[preferredLanguage].Run
		fmt.Printf("Configured language: %s\n", preferredLanguage)
		fmt.Println("Available run actions:")
		for _, action := range sortedKeys(runMap) {
			fmt.Printf("  %s -> %s\n", action, runMap[action])
		}
		return
	}

	lang, err := detectProjectLanguage(cwd, languages)
	if err == nil {
		runMap := languages[lang].Run
		fmt.Printf("Detected language: %s\n", lang)
		fmt.Println("Available run actions:")
		for _, action := range sortedKeys(runMap) {
			fmt.Printf("  %s -> %s\n", action, runMap[action])
		}
		return
	}

	fmt.Printf("Unable to auto-detect project language: %v\n", err)
	fmt.Println("Available run actions by language:")
	for _, language := range sortedKeysLang(languages) {
		fmt.Printf("  [%s]\n", language)
		for _, action := range sortedKeys(languages[language].Run) {
			fmt.Printf("    %s -> %s\n", action, languages[language].Run[action])
		}
	}
}

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

		out[normalizedLang] = appconfig.LanguageConfig{Run: runMap}
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
		merged[lang] = appconfig.LanguageConfig{Run: copiedRun}
	}

	for lang, langCfg := range normalizeLanguages(project) {
		base := merged[lang]
		if base.Run == nil {
			base.Run = map[string]string{}
		}
		for action, command := range langCfg.Run {
			base.Run[action] = command
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

func splitRunArgsAndOptions(tokens []string) ([]string, []string) {
	for i, token := range tokens {
		if token == "--" {
			return tokens[:i], tokens[i+1:]
		}
	}
	return tokens, nil
}

func renderRunCommand(template string, arguments, options []string) string {
	argsValue := joinShellEscaped(arguments)
	optionsValue := joinShellEscaped(options)
	hasArgsPlaceholder := strings.Contains(template, "{args}") || strings.Contains(template, "{arguments}")
	hasOptionsPlaceholder := strings.Contains(template, "{options}")

	replaced := strings.NewReplacer(
		"{args}", argsValue,
		"{arguments}", argsValue,
		"{options}", optionsValue,
	).Replace(template)

	// Backward-compatible behavior: if template omits placeholders,
	// append the corresponding values to the end.
	extraParts := make([]string, 0, 2)
	if !hasArgsPlaceholder && argsValue != "" {
		extraParts = append(extraParts, argsValue)
	}
	if !hasOptionsPlaceholder && optionsValue != "" {
		extraParts = append(extraParts, optionsValue)
	}
	if len(extraParts) > 0 {
		replaced = strings.TrimSpace(replaced + " " + strings.Join(extraParts, " "))
	}

	return strings.TrimSpace(replaced)
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
