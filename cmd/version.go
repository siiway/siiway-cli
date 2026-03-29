package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/SiiWay/siiway-cli/internal/appconfig"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(versionCmd)
}

var versionCmd = &cobra.Command{
	Use:                "version [language] <version> [-- options...]",
	Short:              "Manage language runtime versions",
	DisableFlagParsing: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		listMode, remainingArgs, err := parseVersionInvocation(args)
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
			return fmt.Errorf("no languages configuration found in global config or .siiway.yaml")
		}

		if listMode {
			if len(remainingArgs) > 0 {
				return fmt.Errorf("--list (-l) does not accept language/version arguments")
			}
			printAvailableVersionBackends(cwd, projectCfg.Language, languages)
			return nil
		}

		language, targetVersion, options, err := resolveVersionInvocation(cwd, projectCfg.Language, languages, remainingArgs)
		if err != nil {
			return err
		}

		versionCfg := languages[language].Version
		commandTemplate, err := resolveVersionCommandTemplate(language, versionCfg)
		if err != nil {
			return err
		}

		rendered := renderVersionCommand(commandTemplate, targetVersion, options)
		if strings.TrimSpace(rendered) == "" {
			return fmt.Errorf("resolved version command is empty for language %q", language)
		}

		execCmd := exec.CommandContext(cmd.Context(), "sh", "-c", rendered)
		execCmd.Stdout = os.Stdout
		execCmd.Stderr = os.Stderr
		execCmd.Stdin = os.Stdin
		execCmd.Dir = cwd
		if err := execCmd.Run(); err != nil {
			return fmt.Errorf("version command failed: %w", err)
		}

		return nil
	},
}

func parseVersionInvocation(args []string) (bool, []string, error) {
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
				return false, nil, fmt.Errorf("unknown flag for version: %s", token)
			}
			return listMode, remaining, nil
		}
	}

	return listMode, remaining, nil
}

func resolveVersionInvocation(cwd, preferredLanguage string, languages map[string]appconfig.LanguageConfig, args []string) (string, string, []string, error) {
	if len(args) == 0 {
		return "", "", nil, fmt.Errorf("version is required")
	}

	positional, options := splitArgsAndOptions(args)
	if len(positional) == 0 {
		return "", "", nil, fmt.Errorf("version is required")
	}
	if len(positional) > 2 {
		return "", "", nil, fmt.Errorf("too many arguments: expected [language] <version>")
	}

	if len(positional) == 1 {
		language, err := resolveLanguage(cwd, preferredLanguage, languages)
		if err != nil {
			return "", "", nil, err
		}
		return language, strings.TrimSpace(positional[0]), options, nil
	}

	language := strings.ToLower(strings.TrimSpace(positional[0]))
	if _, ok := languages[language]; !ok {
		return "", "", nil, fmt.Errorf("unknown language for version command: %s (available: %s)", language, strings.Join(sortedKeysLang(languages), ", "))
	}

	if preferredLanguage != "" && preferredLanguage != language {
		return "", "", nil, fmt.Errorf(".siiway.yaml configured language is %q; cannot override with %q", preferredLanguage, language)
	}

	return language, strings.TrimSpace(positional[1]), options, nil
}

func resolveVersionCommandTemplate(language string, cfg appconfig.LanguageVersionConfig) (string, error) {
	if strings.TrimSpace(cfg.Use) != "" {
		return strings.TrimSpace(cfg.Use), nil
	}

	switch strings.ToLower(strings.TrimSpace(cfg.Backend)) {
	case "uv":
		return "uv python pin {version}", nil
	case "nvm":
		return "nvm install {version} && nvm use {version}", nil
	case "":
		return "", fmt.Errorf("version command not configured for language %q: set languages.%s.version.use or backend", language, language)
	default:
		return "", fmt.Errorf("unsupported version backend %q for language %q; set languages.%s.version.use", cfg.Backend, language, language)
	}
}

func renderVersionCommand(template, targetVersion string, options []string) string {
	targetVersion = strings.TrimSpace(targetVersion)
	optionsValue := joinShellEscaped(options)
	hasOptionsPlaceholder := strings.Contains(template, "{options}")

	replaced := strings.NewReplacer(
		"{version}", shellQuote(targetVersion),
		"{options}", optionsValue,
	).Replace(template)

	if !hasOptionsPlaceholder && optionsValue != "" {
		replaced = strings.TrimSpace(replaced + " " + optionsValue)
	}

	return strings.TrimSpace(replaced)
}

func printAvailableVersionBackends(cwd, preferredLanguage string, languages map[string]appconfig.LanguageConfig) {
	if preferredLanguage != "" {
		preferredLanguage = strings.ToLower(strings.TrimSpace(preferredLanguage))
		if _, ok := languages[preferredLanguage]; !ok {
			fmt.Printf("Configured language %q was not found in languages\n", preferredLanguage)
			return
		}
		printSingleLanguageVersionConfig(preferredLanguage, languages[preferredLanguage])
		return
	}

	lang, err := detectProjectLanguage(cwd, languages)
	if err == nil {
		printSingleLanguageVersionConfig(lang, languages[lang])
		return
	}

	fmt.Printf("Unable to auto-detect project language: %v\n", err)
	fmt.Println("Available version backends by language:")
	for _, language := range sortedKeysLang(languages) {
		printSingleLanguageVersionConfig(language, languages[language])
	}
}

func printSingleLanguageVersionConfig(language string, cfg appconfig.LanguageConfig) {
	backend := cfg.Version.Backend
	if backend == "" {
		backend = "(custom-only)"
	}
	fmt.Printf("  [%s] backend=%s\n", language, backend)
	commandTemplate, err := resolveVersionCommandTemplate(language, cfg.Version)
	if err != nil {
		fmt.Printf("    command -> %v\n", err)
		return
	}
	fmt.Printf("    command -> %s\n", commandTemplate)
}
