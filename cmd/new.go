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

var githubToken string

func init() {
	newCmd.Flags().StringVar(&githubToken, "token", "", "GitHub token for private repositories and higher API rate limits")
	rootCmd.AddCommand(newCmd)
}

var newCmd = &cobra.Command{
	Use:   "new <template_name>@<version> <project_name>",
	Short: "Create a new project from a template",
	Args:  cobra.ExactArgs(2),
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

		templateName, version, err := parseTemplateSpecifier(args[0])
		if err != nil {
			return err
		}
		projectName := strings.TrimSpace(args[1])

		selected, err := findTemplateByName(templates, templateName)
		if err != nil {
			return err
		}

		resolvedBranch, err := resolveTemplateBranch(selected.RepoURL, version, token)
		if err != nil {
			return err
		}
		selected.Branch = resolvedBranch

		finalProjectName := projectName

		targetDir := filepath.Clean(finalProjectName)
		if err := validateTargetDir(targetDir); err != nil {
			return err
		}

		if err := confirmCreation(selected, finalProjectName, targetDir); err != nil {
			return err
		}

		if err := cloneTemplate(selected, targetDir, token); err != nil {
			return err
		}

		fmt.Printf("\nProject initialized in %s\n", targetDir)
		fmt.Printf("Next steps:\n  cd %s\n  git init\n\n", targetDir)
		return nil
	},
}

func parseTemplateSpecifier(spec string) (string, string, error) {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return "", "", errors.New("template specifier is required: <template_name>@<version>")
	}

	parts := strings.SplitN(spec, "@", 2)
	if len(parts) != 2 {
		return "", "", errors.New("invalid template specifier, expected <template_name>@<version>")
	}

	templateName := strings.TrimSpace(parts[0])
	version := strings.TrimSpace(parts[1])
	if templateName == "" || version == "" {
		return "", "", errors.New("invalid template specifier, template name and version are required")
	}

	return templateName, version, nil
}

func findTemplateByName(templates []registry.Template, templateName string) (registry.Template, error) {
	for _, t := range templates {
		if strings.EqualFold(strings.TrimSpace(t.Name), strings.TrimSpace(templateName)) {
			return t, nil
		}
	}
	return registry.Template{}, fmt.Errorf("template not found: %s", templateName)
}

func resolveTemplateBranch(repoURL, version, token string) (string, error) {
	v := strings.TrimSpace(version)
	if strings.EqualFold(v, "latest") {
		return "main", nil
	}

	if strings.HasPrefix(strings.ToLower(v), "v") {
		branches, err := listRemoteBranches(repoURL, token)
		if err != nil {
			return "", fmt.Errorf("failed resolving version %q to branch: %w", v, err)
		}

		if matched := matchVersionBranch(v, branches); matched != "" {
			return matched, nil
		}

		return "", fmt.Errorf("no matching branch found for version %q in %s", v, repoURL)
	}

	return v, nil
}

func listRemoteBranches(repoURL, token string) ([]string, error) {
	args := []string{}
	if strings.TrimSpace(token) != "" && strings.HasPrefix(strings.ToLower(repoURL), "https://") {
		credential := base64.StdEncoding.EncodeToString([]byte("x-access-token:" + token))
		args = append(args, "-c", "http.extraHeader=AUTHORIZATION: basic "+credential)
	}
	args = append(args, "ls-remote", "--heads", repoURL)

	cmd := exec.Command("git", args...)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("git ls-remote failed: %s", strings.TrimSpace(string(out)))
	}

	branches := make([]string, 0)
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		const prefix = "refs/heads/"
		if strings.HasPrefix(fields[1], prefix) {
			branches = append(branches, strings.TrimPrefix(fields[1], prefix))
		}
	}

	return branches, nil
}

func matchVersionBranch(version string, branches []string) string {
	base := strings.TrimPrefix(version, "v")
	candidates := []string{
		version,
		base,
		"release/" + version,
		"release/" + base,
		"releases/" + version,
		"releases/" + base,
		"release-" + version,
		"release-" + base,
	}

	for _, candidate := range candidates {
		for _, branch := range branches {
			if branch == candidate {
				return branch
			}
		}
	}

	return ""
}

func isInteractive() bool {
	info, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
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
