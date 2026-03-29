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

		if listMode {
			if len(remainingArgs) > 0 {
				return fmt.Errorf("--list (-l) does not accept action or arguments")
			}
			printAvailableRunActions(cwd, projectCfg.Language, languages)
			return nil
		}
		if len(remainingArgs) == 0 {
			return fmt.Errorf("action is required")
		}

		lang, err := resolveLanguage(cwd, projectCfg.Language, languages)
		if err != nil {
			return err
		}

		action := strings.ToLower(strings.TrimSpace(remainingArgs[0]))
		runMap := languages[lang].Run
		commandTemplate, ok := runMap[action]
		if !ok {
			return fmt.Errorf("run action not found for language %q: %s (available: %s)", lang, action, strings.Join(sortedKeys(runMap), ", "))
		}

		arguments, options := splitArgsAndOptions(remainingArgs[1:])
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
		preferredLanguage = strings.ToLower(strings.TrimSpace(preferredLanguage))
		if _, ok := languages[preferredLanguage]; !ok {
			fmt.Printf("Configured language %q was not found in languages\n", preferredLanguage)
			return
		}
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
