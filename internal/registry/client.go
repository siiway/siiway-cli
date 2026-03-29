package registry

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	registryOwner = "siiway"
	registryRepo  = "cli-templates"
	registryRef   = "main"
	registryPath  = "templates.yaml"
)

var metadataFileCandidates = []string{
	"templates.yaml",
	"registry/templates.yaml",
}

// SourcePath returns the primary GitHub Contents API endpoint for registry metadata.
func SourcePath() string {
	return SourcePathFor(DefaultSource())
}

// Source describes a GitHub repository location for templates metadata.
type Source struct {
	Owner string
	Repo  string
	Ref   string
	Path  string
}

// DefaultSource returns the built-in registry source configuration.
func DefaultSource() Source {
	return Source{
		Owner: registryOwner,
		Repo:  registryRepo,
		Ref:   registryRef,
		Path:  registryPath,
	}
}

func normalizeSource(source Source) Source {
	normalized := source
	normalized.Owner = strings.TrimSpace(normalized.Owner)
	normalized.Repo = strings.TrimSpace(normalized.Repo)
	normalized.Ref = strings.TrimSpace(normalized.Ref)
	normalized.Path = strings.Trim(strings.TrimSpace(normalized.Path), "/")

	defaults := DefaultSource()
	if normalized.Owner == "" {
		normalized.Owner = defaults.Owner
	}
	if normalized.Repo == "" {
		normalized.Repo = defaults.Repo
	}
	if normalized.Ref == "" {
		normalized.Ref = defaults.Ref
	}
	if normalized.Path == "" {
		normalized.Path = defaults.Path
	}

	return normalized
}

// SourcePathFor builds the GitHub Contents API URL for the given source.
func SourcePathFor(source Source) string {
	s := normalizeSource(source)
	return fmt.Sprintf(
		"https://api.github.com/repos/%s/%s/contents/%s?ref=%s",
		s.Owner,
		s.Repo,
		s.Path,
		s.Ref,
	)
}

type githubContentResp struct {
	Type     string `json:"type"`
	Encoding string `json:"encoding"`
	Content  string `json:"content"`
}

// Client is a registry client for fetching template metadata
// from the SiiWay CLI registry repository.
type Client struct {
	httpClient *http.Client
	token      string
	source     Source
}

// Template represents a project template with metadata about
// the template repository and its source code.
type Template struct {
	Name                  string                 `json:"name" yaml:"name"`
	Description           string                 `json:"description" yaml:"description"`
	RepoURL               string                 `json:"repo_url" yaml:"repo_url"`
	Repository            string                 `json:"repository" yaml:"repository"`
	Repo                  string                 `json:"repo" yaml:"repo"`
	URL                   string                 `json:"url" yaml:"url"`
	Branch                string                 `json:"branch" yaml:"branch"`
	Path                  string                 `json:"path" yaml:"path"`
	Replace               TemplateReplace        `json:"replace" yaml:"replace"`
	ProjectNameRegexRules []ProjectNameRegexRule `json:"project_name_regex_rules" yaml:"project_name_regex_rules"`
}

// TemplateReplace defines shorthand replacement settings from templates.yaml.
type TemplateReplace struct {
	ProjectName string `json:"project_name" yaml:"project_name"`
}

// ProjectNameRegexRule defines one regex replacement applied to template files.
type ProjectNameRegexRule struct {
	File        string `json:"file" yaml:"file"`
	FilePattern string `json:"file_pattern" yaml:"file_pattern"`
	Pattern     string `json:"pattern" yaml:"pattern"`
	Replacement string `json:"replacement" yaml:"replacement"`
}

// NewClient creates and returns a new registry client
// with a default HTTP timeout of 12 seconds.
func NewClient(token string) *Client {
	return NewClientWithSource(token, DefaultSource())
}

// NewClientWithSource creates a new registry client for the provided source.
func NewClientWithSource(token string, source Source) *Client {
	return &Client{
		httpClient: &http.Client{Timeout: 12 * time.Second},
		token:      strings.TrimSpace(token),
		source:     normalizeSource(source),
	}
}

// SourcePath returns the primary metadata URL for this client's registry source.
func (c *Client) SourcePath() string {
	return SourcePathFor(c.source)
}

// FetchTemplates retrieves the list of available templates from the registry.
// It only reads templates.yaml online and parses it.
func (c *Client) FetchTemplates(ctx context.Context) ([]Template, error) {
	templates, err := c.fetchFromFixedContentsAPI(ctx)
	if err == nil && len(templates) > 0 {
		return templates, nil
	}
	fixedContentsErr := err

	templates, err = c.fetchFromContentsAPI(ctx)
	if err == nil && len(templates) > 0 {
		return templates, nil
	}
	contentsErr := err

	templates, err = c.fetchFromTree(ctx)
	if err == nil && len(templates) > 0 {
		return templates, nil
	}
	treeErr := err

	if fixedContentsErr != nil || contentsErr != nil || treeErr != nil {
		return nil, fmt.Errorf(
			"online registry fetch failed (contents-fixed: %v; contents-fallback: %v; tree-fallback: %v)",
			fixedContentsErr,
			contentsErr,
			treeErr,
		)
	}

	return nil, errors.New("unable to discover template metadata file in registry")
}

func (c *Client) fetchFromFixedContentsAPI(ctx context.Context) ([]Template, error) {
	apiURL := c.SourcePath()
	body, status, err := c.fetch(ctx, apiURL)
	if err != nil {
		return nil, err
	}
	if status == http.StatusNotFound {
		return nil, errors.New("registry metadata file not found")
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("request failed with status %d for %s", status, apiURL)
	}

	var content githubContentResp
	if err := json.Unmarshal(body, &content); err != nil {
		return nil, fmt.Errorf("failed parsing github contents response: %w", err)
	}
	if content.Type != "file" || strings.ToLower(content.Encoding) != "base64" {
		return nil, fmt.Errorf("unexpected github contents response for %s", c.source.Path)
	}

	decoded, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(content.Content, "\n", ""))
	if err != nil {
		return nil, fmt.Errorf("failed base64 decoding metadata content: %w", err)
	}

	templates, err := parseTemplates(decoded, c.source.Path)
	if err != nil {
		return nil, fmt.Errorf("failed parsing %s: %w", c.source.Path, err)
	}
	normalized := normalizeTemplates(templates)
	if len(normalized) == 0 {
		return nil, errors.New("no templates found in fixed contents registry metadata")
	}

	return normalized, nil
}

func (c *Client) fetchFromContentsAPI(ctx context.Context) ([]Template, error) {
	branches := []string{c.source.Ref, "main", "master"}

	for _, branch := range branches {
		for _, fileName := range fileCandidatesForBranch(c.source, branch) {
			apiURL := fmt.Sprintf(
				"https://api.github.com/repos/%s/%s/contents/%s?ref=%s",
				c.source.Owner,
				c.source.Repo,
				fileName,
				branch,
			)

			body, status, err := c.fetch(ctx, apiURL)
			if err != nil {
				return nil, err
			}
			if status == http.StatusNotFound {
				continue
			}
			if status != http.StatusOK {
				return nil, fmt.Errorf("request failed with status %d for %s", status, apiURL)
			}

			var content githubContentResp
			if err := json.Unmarshal(body, &content); err != nil {
				return nil, fmt.Errorf("failed parsing github contents response: %w", err)
			}
			if content.Type != "file" || strings.ToLower(content.Encoding) != "base64" {
				continue
			}

			decoded, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(content.Content, "\n", ""))
			if err != nil {
				return nil, fmt.Errorf("failed base64 decoding metadata content: %w", err)
			}

			templates, err := parseTemplates(decoded, fileName)
			if err != nil {
				return nil, fmt.Errorf("failed parsing %s: %w", fileName, err)
			}
			normalized := normalizeTemplates(templates)
			if len(normalized) > 0 {
				return normalized, nil
			}
		}
	}

	return nil, errors.New("unable to fetch parseable metadata from github contents api")
}

func fileCandidatesForBranch(source Source, branch string) []string {
	preferredPath := strings.Trim(strings.TrimSpace(source.Path), "/")
	if preferredPath == "" {
		preferredPath = registryPath
	}

	candidates := []string{preferredPath}
	defaultSource := DefaultSource()
	if branch == defaultSource.Ref {
		candidates = append(candidates, "templates.yaml", "registry/templates.yaml")
	} else {
		candidates = append(candidates, metadataFileCandidates...)
	}

	seen := map[string]struct{}{}
	uniq := make([]string, 0, len(candidates))
	for _, c := range candidates {
		if _, ok := seen[c]; ok {
			continue
		}
		seen[c] = struct{}{}
		uniq = append(uniq, c)
	}

	return uniq
}

func (c *Client) fetchFromGitClone(ctx context.Context) ([]Template, error) {
	if _, err := exec.LookPath("git"); err != nil {
		return nil, errors.New("git is required to access registry")
	}

	cloneURLs := []string{
		fmt.Sprintf("https://github.com/%s/%s.git", c.source.Owner, c.source.Repo),
	}
	if c.token == "" {
		cloneURLs = append([]string{fmt.Sprintf("git@github.com:%s/%s.git", c.source.Owner, c.source.Repo)}, cloneURLs...)
	}

	var cloneErrs []string
	for _, cloneURL := range cloneURLs {
		tempDir, err := os.MkdirTemp("", "siiway-cli-registry-")
		if err != nil {
			return nil, err
		}
		defer os.RemoveAll(tempDir)

		cloneCmd := c.newGitCloneCmd(ctx, cloneURL, tempDir)
		if output, err := cloneCmd.CombinedOutput(); err != nil {
			cloneErrs = append(cloneErrs, fmt.Sprintf("%s: %s", cloneURL, strings.TrimSpace(string(output))))
			continue
		}

		templates, err := parseMetadataFromDir(tempDir)
		if err != nil {
			return nil, err
		}
		if len(templates) > 0 {
			return templates, nil
		}
	}

	if len(cloneErrs) > 0 {
		return nil, fmt.Errorf("failed cloning registry repository: %s", strings.Join(cloneErrs, " | "))
	}

	return nil, errors.New("unable to clone registry repository")
}

func parseMetadataFromDir(root string) ([]Template, error) {
	for _, fileName := range metadataFileCandidates {
		fullPath := filepath.Join(root, fileName)
		data, err := os.ReadFile(fullPath)
		if err != nil {
			continue
		}

		templates, err := parseTemplates(data, fileName)
		if err != nil {
			return nil, fmt.Errorf("failed parsing %s: %w", fileName, err)
		}
		normalized := normalizeTemplates(templates)
		if len(normalized) > 0 {
			return normalized, nil
		}
	}

	pathMatcher := regexp.MustCompile(`(?i)(template|registry|metadata).+\.(json|ya?ml)$`)
	var found []Template

	err := filepath.WalkDir(root, func(filePath string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if d.IsDir() {
			if d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}

		relPath, err := filepath.Rel(root, filePath)
		if err != nil || !pathMatcher.MatchString(strings.ToLower(relPath)) {
			return nil
		}

		data, err := os.ReadFile(filePath)
		if err != nil {
			return nil
		}

		templates, err := parseTemplates(data, relPath)
		if err != nil {
			return nil
		}
		found = append(found, templates...)
		return nil
	})
	if err != nil {
		return nil, err
	}

	normalized := normalizeTemplates(found)
	if len(normalized) > 0 {
		return normalized, nil
	}

	return nil, errors.New("no template metadata found in cloned registry")
}

func (c *Client) fetchFromTree(ctx context.Context) ([]Template, error) {
	branches := []string{c.source.Ref, "main", "master"}
	pathMatcher := regexp.MustCompile(`(?i)(template|registry|metadata).+\.(json|ya?ml)$`)

	type treeNode struct {
		Path string `json:"path"`
		SHA  string `json:"sha"`
		Type string `json:"type"`
	}
	type treeResp struct {
		Tree []treeNode `json:"tree"`
	}
	type blobResp struct {
		Encoding string `json:"encoding"`
		Content  string `json:"content"`
	}

	for _, branch := range branches {
		apiURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/git/trees/%s?recursive=1", c.source.Owner, c.source.Repo, branch)
		body, status, err := c.fetch(ctx, apiURL)
		if err != nil {
			return nil, err
		}
		if status == http.StatusNotFound {
			continue
		}
		if status != http.StatusOK {
			return nil, fmt.Errorf("request failed with status %d for %s", status, apiURL)
		}

		var tree treeResp
		if err := json.Unmarshal(body, &tree); err != nil {
			return nil, fmt.Errorf("failed parsing github tree response: %w", err)
		}

		for _, node := range tree.Tree {
			if node.Type != "blob" || !pathMatcher.MatchString(node.Path) {
				continue
			}

			blobURL := fmt.Sprintf(
				"https://api.github.com/repos/%s/%s/git/blobs/%s",
				c.source.Owner,
				c.source.Repo,
				node.SHA,
			)

			blobBody, blobStatus, err := c.fetch(ctx, blobURL)
			if err != nil {
				return nil, err
			}
			if blobStatus != http.StatusOK {
				continue
			}

			var blob blobResp
			if err := json.Unmarshal(blobBody, &blob); err != nil {
				continue
			}
			if strings.ToLower(blob.Encoding) != "base64" {
				continue
			}

			decoded, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(blob.Content, "\n", ""))
			if err != nil {
				continue
			}

			templates, err := parseTemplates(decoded, node.Path)
			if err != nil {
				continue
			}
			normalized := normalizeTemplates(templates)
			if len(normalized) > 0 {
				return normalized, nil
			}
		}
	}

	return nil, errors.New("unable to find parseable metadata from github tree")
}

func (c *Client) fetch(ctx context.Context, url string) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "siiway-cli")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, 0, err
	}
	return body, resp.StatusCode, nil
}

func (c *Client) newGitCloneCmd(ctx context.Context, cloneURL, targetDir string) *exec.Cmd {
	args := []string{}
	if c.token != "" && strings.HasPrefix(strings.ToLower(cloneURL), "https://") {
		credential := base64.StdEncoding.EncodeToString([]byte("x-access-token:" + c.token))
		args = append(args, "-c", "http.extraHeader=AUTHORIZATION: basic "+credential)
	}
	args = append(args, "clone", "--depth", "1", cloneURL, targetDir)
	cloneCmd := exec.CommandContext(ctx, "git", args...)
	cloneCmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	return cloneCmd
}

func parseTemplates(data []byte, fileName string) ([]Template, error) {
	ext := strings.ToLower(path.Ext(fileName))
	if ext == ".yaml" || ext == ".yml" {
		return parseYAMLTemplates(data)
	}
	return parseJSONTemplates(data)
}

func parseJSONTemplates(data []byte) ([]Template, error) {
	type wrapped struct {
		Templates []Template `json:"templates"`
	}

	var list []Template
	if err := json.Unmarshal(data, &list); err == nil {
		return list, nil
	}

	var w wrapped
	if err := json.Unmarshal(data, &w); err == nil && len(w.Templates) > 0 {
		return w.Templates, nil
	}

	var byName map[string]Template
	if err := json.Unmarshal(data, &byName); err == nil && len(byName) > 0 {
		out := make([]Template, 0, len(byName))
		for name, tpl := range byName {
			if strings.TrimSpace(tpl.Name) == "" {
				tpl.Name = name
			}
			out = append(out, tpl)
		}
		return out, nil
	}

	var repoByName map[string]string
	if err := json.Unmarshal(data, &repoByName); err == nil && len(repoByName) > 0 {
		out := make([]Template, 0, len(repoByName))
		for name, repo := range repoByName {
			out = append(out, Template{Name: name, RepoURL: repo})
		}
		return out, nil
	}

	return nil, errors.New("unsupported json format")
}

func parseYAMLTemplates(data []byte) ([]Template, error) {
	type wrapped struct {
		Templates []Template `yaml:"templates"`
	}

	var list []Template
	if err := yaml.Unmarshal(data, &list); err == nil && len(list) > 0 {
		return list, nil
	}

	var w wrapped
	if err := yaml.Unmarshal(data, &w); err == nil && len(w.Templates) > 0 {
		return w.Templates, nil
	}

	var byName map[string]Template
	if err := yaml.Unmarshal(data, &byName); err == nil && len(byName) > 0 {
		out := make([]Template, 0, len(byName))
		for name, tpl := range byName {
			if strings.TrimSpace(tpl.Name) == "" {
				tpl.Name = name
			}
			out = append(out, tpl)
		}
		return out, nil
	}

	var repoByName map[string]string
	if err := yaml.Unmarshal(data, &repoByName); err == nil && len(repoByName) > 0 {
		out := make([]Template, 0, len(repoByName))
		for name, repo := range repoByName {
			out = append(out, Template{Name: name, RepoURL: repo})
		}
		return out, nil
	}

	var generic any
	if err := yaml.Unmarshal(data, &generic); err == nil {
		out := collectTemplatesFromGeneric(generic, "")
		if len(out) > 0 {
			return out, nil
		}
	}

	return nil, errors.New("unsupported yaml format")
}

func collectTemplatesFromGeneric(node any, defaultName string) []Template {
	result := []Template{}

	switch v := node.(type) {
	case map[string]any:
		if t, ok := templateFromMap(v, defaultName); ok {
			result = append(result, t)
		}
		for key, child := range v {
			result = append(result, collectTemplatesFromGeneric(child, key)...)
		}
	case map[any]any:
		converted := make(map[string]any, len(v))
		for key, child := range v {
			converted[fmt.Sprint(key)] = child
		}
		result = append(result, collectTemplatesFromGeneric(converted, defaultName)...)
	case []any:
		for _, child := range v {
			result = append(result, collectTemplatesFromGeneric(child, defaultName)...)
		}
	}

	return result
}

func templateFromMap(m map[string]any, defaultName string) (Template, bool) {
	repo := firstNonEmpty(
		stringValue(m["repo_url"]),
		stringValue(m["repository"]),
		stringValue(m["repo"]),
		stringValue(m["url"]),
	)
	if strings.TrimSpace(repo) == "" {
		return Template{}, false
	}

	name := strings.TrimSpace(stringValue(m["name"]))
	if name == "" {
		name = strings.TrimSpace(defaultName)
	}

	return Template{
		Name:        name,
		Description: strings.TrimSpace(stringValue(m["description"])),
		RepoURL:     repo,
		Branch:      strings.TrimSpace(stringValue(m["branch"])),
		Path:        strings.TrimSpace(stringValue(m["path"])),
		Replace: TemplateReplace{
			ProjectName: projectNameReplaceFromAny(m["replace"]),
		},
		ProjectNameRegexRules: normalizeProjectNameRegexRules(regexRulesFromAny(firstNonNil(
			m["project_name_regex_rules"],
			m["name_regex_rules"],
			m["regex_rules"],
		))),
	}, true
}

func firstNonNil(values ...any) any {
	for _, v := range values {
		if v != nil {
			return v
		}
	}
	return nil
}

func regexRulesFromAny(v any) []ProjectNameRegexRule {
	if v == nil {
		return nil
	}

	rules := []ProjectNameRegexRule{}
	switch t := v.(type) {
	case []ProjectNameRegexRule:
		return t
	case []any:
		for _, item := range t {
			rule, ok := regexRuleFromAny(item)
			if ok {
				rules = append(rules, rule)
			}
		}
	}

	return rules
}

func regexRuleFromAny(v any) (ProjectNameRegexRule, bool) {
	switch t := v.(type) {
	case map[string]any:
		rule := ProjectNameRegexRule{
			File:        strings.TrimSpace(stringValue(t["file"])),
			FilePattern: strings.TrimSpace(stringValue(t["file_pattern"])),
			Pattern:     strings.TrimSpace(stringValue(t["pattern"])),
			Replacement: stringValue(t["replacement"]),
		}
		if rule.Pattern == "" {
			return ProjectNameRegexRule{}, false
		}
		return rule, true
	case map[any]any:
		converted := make(map[string]any, len(t))
		for key, value := range t {
			converted[fmt.Sprint(key)] = value
		}
		return regexRuleFromAny(converted)
	default:
		return ProjectNameRegexRule{}, false
	}
}

func normalizeProjectNameRegexRules(raw []ProjectNameRegexRule) []ProjectNameRegexRule {
	out := make([]ProjectNameRegexRule, 0, len(raw))
	for _, rule := range raw {
		normalized := ProjectNameRegexRule{
			File:        strings.Trim(strings.TrimSpace(rule.File), "/"),
			FilePattern: strings.TrimSpace(rule.FilePattern),
			Pattern:     strings.TrimSpace(rule.Pattern),
			Replacement: rule.Replacement,
		}
		if normalized.Pattern == "" {
			continue
		}
		if normalized.Replacement == "" {
			normalized.Replacement = "{{project_name}}"
		}
		out = append(out, normalized)
	}
	return out
}

func projectNameReplaceFromAny(v any) string {
	if v == nil {
		return ""
	}

	switch t := v.(type) {
	case map[string]any:
		return strings.TrimSpace(stringValue(t["project_name"]))
	case map[any]any:
		converted := make(map[string]any, len(t))
		for key, value := range t {
			converted[fmt.Sprint(key)] = value
		}
		return projectNameReplaceFromAny(converted)
	default:
		return ""
	}
}

func ruleFromProjectNameReplace(raw string) (ProjectNameRegexRule, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ProjectNameRegexRule{}, false
	}

	parts := strings.SplitN(raw, ":", 2)
	if len(parts) != 2 {
		return ProjectNameRegexRule{}, false
	}

	file := strings.Trim(strings.TrimSpace(parts[0]), "/")
	pattern := strings.TrimSpace(parts[1])
	if file == "" || pattern == "" {
		return ProjectNameRegexRule{}, false
	}

	return ProjectNameRegexRule{
		File:        file,
		Pattern:     pattern,
		Replacement: "{{project_name}}",
	}, true
}

func stringValue(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case fmt.Stringer:
		return t.String()
	default:
		return ""
	}
}

func normalizeTemplates(raw []Template) []Template {
	seen := map[string]struct{}{}
	out := make([]Template, 0, len(raw))

	for _, tpl := range raw {
		repo := firstNonEmpty(tpl.RepoURL, tpl.Repository, tpl.Repo, tpl.URL)
		repo = normalizeRepoURL(repo)
		if repo == "" {
			continue
		}

		name := strings.TrimSpace(tpl.Name)
		if name == "" {
			parts := strings.Split(strings.TrimSuffix(repo, ".git"), "/")
			name = parts[len(parts)-1]
		}

		branch := strings.TrimSpace(tpl.Branch)
		if branch == "" {
			branch = "main"
		}

		rules := normalizeProjectNameRegexRules(tpl.ProjectNameRegexRules)
		if len(rules) == 0 {
			if shorthandRule, ok := ruleFromProjectNameReplace(tpl.Replace.ProjectName); ok {
				rules = []ProjectNameRegexRule{shorthandRule}
			}
		}

		key := strings.ToLower(name + "|" + repo)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}

		out = append(out, Template{
			Name:                  name,
			Description:           strings.TrimSpace(tpl.Description),
			RepoURL:               repo,
			Branch:                branch,
			Path:                  strings.TrimSpace(tpl.Path),
			Replace:               tpl.Replace,
			ProjectNameRegexRules: rules,
		})
	}

	sort.Slice(out, func(i, j int) bool {
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})

	return out
}

func normalizeRepoURL(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return ""
	}

	if strings.HasPrefix(v, "http://") || strings.HasPrefix(v, "https://") || strings.HasPrefix(v, "git@") {
		return v
	}

	if strings.HasPrefix(v, "github.com/") {
		return "https://" + v
	}

	if strings.Count(v, "/") == 1 {
		return "https://github.com/" + v
	}

	return v
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
