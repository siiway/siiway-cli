package cmd

import (
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/SiiWay/siiway-cli/internal/registry"
	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
)

var templateName string
var githubToken string

func init() {
	newCmd.Flags().StringVarP(&templateName, "template", "t", "", "Template name to use")
	newCmd.Flags().StringVar(&githubToken, "token", "", "GitHub token for private repositories and higher API rate limits")
	rootCmd.AddCommand(newCmd)
}

var newCmd = &cobra.Command{
	Use:   "new [project-name]",
	Short: "Create a new project from a template",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		token := strings.TrimSpace(githubToken)
		if token == "" {
			token = strings.TrimSpace(os.Getenv("GITHUB_TOKEN"))
		}

		client := registry.NewClient(token)
		templates, err := client.FetchTemplates(cmd.Context())
		if err != nil {
			return fmt.Errorf("failed to fetch template metadata from %s: %w", registry.SourcePath(), err)
		}
		if len(templates) == 0 {
			return errors.New("no templates found in registry")
		}

		projectName := ""
		if len(args) == 1 {
			projectName = strings.TrimSpace(args[0])
		}

		selected, finalProjectName, err := runNewWizard(templates, projectName)
		if err != nil {
			return err
		}

		targetDir := filepath.Clean(finalProjectName)

		if err := cloneTemplate(selected, targetDir, token); err != nil {
			return err
		}

		fmt.Printf("\nProject initialized in %s\n", targetDir)
		fmt.Printf("Next steps:\n  cd %s\n  git init\n\n", targetDir)
		return nil
	},
}

func runNewWizard(templates []registry.Template, projectName string) (registry.Template, string, error) {
	selected, err := chooseTemplateTUI(templates, templateName)
	if err != nil {
		return registry.Template{}, "", err
	}

	finalProjectName, err := chooseProjectName(projectName, selected.Name)
	if err != nil {
		return registry.Template{}, "", err
	}

	targetDir := filepath.Clean(finalProjectName)
	if err := validateTargetDir(targetDir); err != nil {
		return registry.Template{}, "", err
	}

	if err := confirmCreation(selected, finalProjectName, targetDir); err != nil {
		return registry.Template{}, "", err
	}

	return selected, finalProjectName, nil
}

func isInteractive() bool {
	info, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

func chooseTemplateTUI(templates []registry.Template, chosen string) (registry.Template, error) {
	if chosen != "" {
		for _, t := range templates {
			if strings.EqualFold(t.Name, chosen) {
				fmt.Printf("Selected template: %s\n", t.Name)
				return t, nil
			}
		}
		return registry.Template{}, fmt.Errorf("template not found: %s", chosen)
	}

	if !isInteractive() {
		return registry.Template{}, errors.New("non-interactive terminal detected; please use --template")
	}

	selectPrompt := promptui.Select{
		Label: "Step 1/3: Select a template",
		Items: templates,
		Size:  12,
		Templates: &promptui.SelectTemplates{
			Label:    "{{ . }}",
			Active:   "▸ {{ .Name | cyan }} - {{ .Description | faint }}",
			Inactive: "  {{ .Name }} - {{ .Description }}",
			Selected: "Selected: {{ .Name | green }}",
		},
		Searcher: func(input string, index int) bool {
			item := templates[index]
			q := strings.ToLower(strings.TrimSpace(input))
			return strings.Contains(strings.ToLower(item.Name), q) || strings.Contains(strings.ToLower(item.Description), q)
		},
	}

	idx, _, err := selectPrompt.Run()
	if err != nil {
		return registry.Template{}, fmt.Errorf("template selection cancelled: %w", err)
	}

	return templates[idx], nil
}

func chooseProjectName(current string, templateName string) (string, error) {
	projectName := strings.TrimSpace(current)
	if projectName != "" {
		return projectName, nil
	}

	if !isInteractive() {
		return "", errors.New("project name is required in non-interactive mode")
	}

	defaultName := strings.TrimSpace(templateName)
	if defaultName == "" {
		defaultName = "my-project"
	}

	prompt := promptui.Prompt{
		Label:   "Step 2/3: Project name",
		Default: defaultName,
		Validate: func(input string) error {
			name := strings.TrimSpace(input)
			if name == "" {
				return errors.New("project name cannot be empty")
			}
			if err := validateTargetDir(filepath.Clean(name)); err != nil {
				return err
			}
			return nil
		},
	}

	name, err := prompt.Run()
	if err != nil {
		return "", fmt.Errorf("project name input cancelled: %w", err)
	}

	return strings.TrimSpace(name), nil
}

func confirmCreation(t registry.Template, projectName, targetDir string) error {
	if !isInteractive() {
		return nil
	}

	fmt.Printf("\nStep 3/3: Confirm configuration\n")
	fmt.Printf("  Template: %s\n", t.Name)
	fmt.Printf("  Repository: %s\n", t.RepoURL)
	fmt.Printf("  Project name: %s\n", projectName)
	fmt.Printf("  Target directory: %s\n\n", targetDir)

	confirm := promptui.Select{
		Label: "Create project now?",
		Items: []string{"Yes", "No"},
		Size:  2,
	}

	idx, _, err := confirm.Run()
	if err != nil {
		return fmt.Errorf("confirmation cancelled: %w", err)
	}
	if idx != 0 {
		return errors.New("operation cancelled")
	}

	return nil
}

func validateTargetDir(targetDir string) error {
	if strings.TrimSpace(targetDir) == "" || targetDir == "." {
		return errors.New("target directory cannot be empty")
	}
	if _, err := os.Stat(targetDir); err == nil {
		return fmt.Errorf("target directory already exists: %s", targetDir)
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("failed checking target directory: %w", err)
	}
	return nil
}

func cloneTemplate(t registry.Template, targetDir string, token string) error {
	if _, err := exec.LookPath("git"); err != nil {
		return errors.New("git is required but not found in PATH")
	}

	branch := strings.TrimSpace(t.Branch)
	if branch == "" {
		branch = "main"
	}

	templatePath := normalizeTemplatePath(t.Path)

	fmt.Printf("\nCloning template %q from %s ...\n", t.Name, t.RepoURL)
	if templatePath != "" {
		fmt.Printf("Using branch %q and path %q\n", branch, templatePath)
	}

	cloneCmd := gitCloneCmd(t.RepoURL, targetDir, branch, token, true, templatePath)
	cloneCmd.Stdout = os.Stdout
	cloneCmd.Stderr = os.Stderr
	if err := cloneCmd.Run(); err != nil {
		// Fallback for templates that omit branch metadata or use a different default branch.
		fallbackCmd := gitCloneCmd(t.RepoURL, targetDir, "", token, false, templatePath)
		fallbackCmd.Stdout = os.Stdout
		fallbackCmd.Stderr = os.Stderr
		if fallbackErr := fallbackCmd.Run(); fallbackErr != nil {
			return fmt.Errorf("git clone failed: %w", err)
		}
	}

	if templatePath != "" {
		if err := enableSparseCheckout(targetDir, templatePath); err != nil {
			return err
		}

		if err := moveSparsePathToRoot(targetDir, templatePath); err != nil {
			return err
		}
	}

	// Remove source history to turn cloned template into a fresh project.
	if err := os.RemoveAll(filepath.Join(targetDir, ".git")); err != nil {
		return fmt.Errorf("failed removing template git history: %w", err)
	}

	return nil
}

func gitCloneCmd(repoURL, targetDir, branch, token string, useBranch bool, templatePath string) *exec.Cmd {
	args := []string{"clone", "--depth", "1"}
	if templatePath != "" {
		args = append(args, "--filter=blob:none", "--sparse")
	}
	if strings.TrimSpace(token) != "" && strings.HasPrefix(strings.ToLower(repoURL), "https://") {
		credential := base64.StdEncoding.EncodeToString([]byte("x-access-token:" + token))
		args = append(args, "-c", "http.extraHeader=AUTHORIZATION: basic "+credential)
	}
	if useBranch && strings.TrimSpace(branch) != "" {
		args = append(args, "--branch", branch)
	}
	args = append(args, repoURL, targetDir)
	cloneCmd := exec.Command("git", args...)
	cloneCmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	return cloneCmd
}

func enableSparseCheckout(targetDir, templatePath string) error {
	setCmd := gitInDirCmd(targetDir, "sparse-checkout", "set", "--no-cone", templatePath)
	setCmd.Stdout = os.Stdout
	setCmd.Stderr = os.Stderr
	if err := setCmd.Run(); err != nil {
		return fmt.Errorf("failed sparse-checkout path %q: %w", templatePath, err)
	}

	checkoutCmd := gitInDirCmd(targetDir, "checkout")
	checkoutCmd.Stdout = os.Stdout
	checkoutCmd.Stderr = os.Stderr
	if err := checkoutCmd.Run(); err != nil {
		return fmt.Errorf("failed checkout sparse path %q: %w", templatePath, err)
	}

	return nil
}

func moveSparsePathToRoot(targetDir, templatePath string) error {
	sourcePath := filepath.Join(targetDir, filepath.FromSlash(templatePath))
	entries, err := os.ReadDir(sourcePath)
	if err != nil {
		return fmt.Errorf("template path not found in repository: %s", templatePath)
	}

	for _, entry := range entries {
		src := filepath.Join(sourcePath, entry.Name())
		dst := filepath.Join(targetDir, entry.Name())
		if err := os.Rename(src, dst); err != nil {
			return fmt.Errorf("failed to move %s to project root: %w", entry.Name(), err)
		}
	}

	if err := os.RemoveAll(filepath.Join(targetDir, strings.Split(templatePath, "/")[0])); err != nil {
		return fmt.Errorf("failed cleaning sparse checkout directories: %w", err)
	}

	return nil
}

func gitInDirCmd(targetDir string, args ...string) *exec.Cmd {
	fullArgs := append([]string{"-C", targetDir}, args...)
	cmd := exec.Command("git", fullArgs...)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	return cmd
}

func normalizeTemplatePath(p string) string {
	p = strings.TrimSpace(strings.ReplaceAll(p, "\\", "/"))
	p = strings.TrimPrefix(p, "./")
	p = strings.TrimPrefix(p, "/")
	p = strings.TrimSuffix(p, "/")
	if p == "." {
		return ""
	}
	return p
}
