# siiway-cli

English | [中文](README.zh-CN.md)

A lightweight CLI for scaffolding projects from remote template registries.

## Features

- Create projects from a template registry hosted on GitHub.
- Support both direct command mode and interactive TUI mode.
- Resolve template versions with simple rules:
  - `latest` -> `main`
  - `v*` -> match an existing remote branch (for example `v1` or `release/v1`).
- Support template subdirectory extraction (`path`) via sparse checkout.
- Support project-name replacement rules defined in `templates.yaml`.
- Support root `.template` file placeholder replacement and automatic rename.
- Support multi-registry aliases in user config.
- Support default GitHub token from CLI config.

## Requirements

- Go 1.22+ (for local build)
- Git (required at runtime for template cloning)

## Install

### Build from source

```bash
go build -o siiway .
```

### Build with scripts

Single-platform build:

```bash
./scripts/build/build.sh
```

Multi-platform build:

```bash
./scripts/build/build-all.sh
```

Output binaries are generated in `bin/` by default.

### One-line install via cli.siiway.org

Windows (PowerShell):

```powershell
irm https://cli.siiway.org/get | iex
```

Linux / macOS:

```bash
curl -fsSL https://cli.siiway.org/get | sh
```

Pin a version:

```bash
curl -fsSL https://cli.siiway.org/get | SIIWAY_VERSION=v1.0.0 sh
```

The install script fetches binaries from GitHub Releases.

## Quick Start

### Interactive mode

```bash
siiway new
```

This starts a TUI flow:

1. Select template
2. Input version
3. Input project name
4. Confirm and create

### Direct mode

```bash
siiway new <template_name>@<version> <project_name>
```

Example:

```bash
siiway new python-service@latest my-python-service
```

### Run project commands

```bash
siiway run <action> [args...] [-- options...]
```

List available run actions from the effective configuration:

```bash
siiway run --list
siiway run -l
```

The `run` command loads subcommand templates from two sources:

- Global config: `~/.config/siiway/config.yaml`
- Project config: `.siiway.yaml` in current working directory (higher priority)

You can set `language` in `.siiway.yaml` to force the current project language. When set, CLI skips auto-detection and can run non-built-in project types as long as `languages.<language>.run.*` is configured.

Config path: `languages.<language>.run.<action>`

Supported placeholders:

- `{args}` (alias: `{arguments}`)
- `{options}`

If placeholders are omitted in a run template, CLI appends args/options to the end automatically.

Example:

```bash
siiway run add typescript -- --clean
```

With:

```yaml
languages:
  node:
    run:
      add: bun add {arguments} {options}
```

Final command:

```bash
bun add typescript --clean
```

With template replacement values:

```bash
siiway new python-service@latest my-python-service \
  --pm uv \
  --project-version 0.1.0 \
  --project-author "Your Name" \
  --project-description "My project"
```

After project creation, the CLI scans all files ending with `.template` in the generated project root, replaces built-in placeholders, then renames files by removing the `.template` suffix.

Built-in placeholders:

- `{pm}`
- `{sw-version}`
- `{project-name}`
- `{project-version}`
- `{project-author}`
- `{project-description}`

## Authentication

GitHub token priority (high to low):

1. `--token` flag
2. `GITHUB_TOKEN` environment variable
3. `github_token` in CLI config

Example:

```bash
siiway new python-service@latest my-service --token <your_token>
```

## Config Command

Show current config:

```bash
siiway config show
```

Set value by key:

```bash
siiway config set <key_name> <value>
```

Reset to defaults:

```bash
siiway config reset
```

### Supported keys

- `token` (alias of `github_token`)
- `github_token`
- `current_registry`
- `registries.<alias>.owner`
- `registries.<alias>.repo`
- `registries.<alias>.ref`
- `registries.<alias>.path`
- `registries.<alias>.source` (format: `owner/repo`)

### Registry examples

Create/update a custom registry alias named `internal`:

```bash
siiway config set registries.internal.source your-org/cli-templates
siiway config set registries.internal.ref main
siiway config set registries.internal.path templates.yaml
siiway config set current_registry internal
```

Set default GitHub token:

```bash
siiway config set github_token <your_token>
```

## Template Metadata (`templates.yaml`)

A minimal template entry:

```yaml
- name: python-service
  description: Python starter project
  repo_url: https://github.com/your-org/python-template
  branch: main
  path: template
```

### Project name replacement rules

You can define regex replacement rules that run after cloning.

Option A: shorthand (`replace.project_name`)

```yaml
- name: python-service
  repo_url: https://github.com/your-org/python-template
  replace:
    project_name: pyproject.toml:siiway-python-template
```

Meaning:

- File: `pyproject.toml`
- Regex pattern: `siiway-python-template`
- Replacement: new project name

Option B: explicit rules (`project_name_regex_rules`)

```yaml
- name: python-service
  repo_url: https://github.com/your-org/python-template
  project_name_regex_rules:
    - file: pyproject.toml
      pattern: siiway-python-template
      replacement: "{{project_name}}"
    - file_pattern: "**/*.md"
      pattern: siiway-python-template
      replacement: "{{project_name}}"
```

Rule fields:

- `file`: target one file (relative path in generated project)
- `file_pattern`: target multiple files by glob
- `pattern`: regex to match
- `replacement`: replacement text (`{{project_name}}` is supported)

## CI Artifacts

GitHub Actions workflow builds binaries for:

- linux/amd64
- linux/arm64
- darwin/amd64
- darwin/arm64
- windows/amd64
- windows/arm64

Artifacts are uploaded per platform by the workflow.

## Release

GitHub Release workflow file: `.github/workflows/release.yml`

- Push tag `v*` (for example `v1.0.0`) to build all binaries and publish release assets.
- You can also trigger the workflow manually with input `tag`.

## Troubleshooting

- If metadata fetch fails, verify registry settings:
  - `owner`, `repo`, `ref`, `path`
- If cloning private repositories fails:
  - Provide a valid token with required repo access.
- If branch resolution for `v*` fails:
  - Ensure matching remote branch exists.

## License

Licensed under the MIT License. See [LICENSE](LICENSE).
