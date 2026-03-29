# siiway-cli

[English](README.md) | 中文

一个轻量级 CLI，用于从远程模板注册中心快速生成项目骨架。

## 功能特性

- 从 GitHub 托管的模板注册中心创建项目。
- 同时支持命令直传模式与交互式 TUI 模式。
- 版本解析规则简单清晰：
  - latest -> main
  - v* -> 匹配远程仓库中已存在的分支（例如 v1 或 release/v1）。
- 支持通过 path 对模板子目录进行稀疏检出。
- 支持在 templates.yaml 中定义项目名正则替换规则。
- 支持根目录 `.template` 文件占位符替换并自动重命名。
- 支持用户配置中的多注册中心别名。
- 支持从配置中读取默认 GitHub Token。

## 环境要求

- Go 1.22+（本地构建需要）
- Git（运行时克隆模板需要）

## 安装

### 从源码构建

```bash
go build -o siiway .
```

### 使用脚本构建

单平台构建：

```bash
./scripts/build/build.sh
```

多平台构建：

```bash
./scripts/build/build-all.sh
```

默认会在 bin/ 目录输出二进制文件。

### 通过 cli.siiway.org 一键安装

Windows（PowerShell）：

```powershell
irm https://cli.siiway.org/get | iex
```

Linux / macOS：

```bash
curl -fsSL https://cli.siiway.org/get | sh
```

指定版本安装：

```bash
curl -fsSL https://cli.siiway.org/get | SIIWAY_VERSION=v1.0.0 sh
```

安装脚本会从 GitHub Releases 下载二进制文件。

## 快速开始

### 交互模式

```bash
siiway new
```

会进入 TUI 流程：

1. 选择模板
2. 输入版本
3. 输入项目名
4. 确认并创建

### 直传模式

```bash
siiway new <template_name>@<version> <project_name>
```

示例：

```bash
siiway new python-service@latest my-python-service
```

### 运行项目命令

```bash
siiway run <action> [args...] [-- options...]
```

查看当前生效配置中的可用 run 子命令：

```bash
siiway run --list
siiway run -l
```

`run` 命令会从两个来源读取子命令模板：

- 全局配置：`~/.config/siiway/config.yaml`
- 项目配置：当前工作目录下 `.siiway.yaml`（优先级更高）

你可以在 `.siiway.yaml` 中设置 `language` 指定当前项目语言。设置后 CLI 会跳过自动检测；只要配置了 `languages.<language>.run.*`，即可支持未内置的项目类型。

配置路径：`languages.<language>.run.<action>`

支持占位符：

- `{args}`（别名：`{arguments}`）
- `{options}`

如果 run 模板中未声明占位符，CLI 会自动将 args/options 追加到命令末尾。

示例：

```bash
siiway run add typescript -- --clean
```

配置为：

```yaml
languages:
  node:
    run:
      add: bun add {arguments} {options}
```

最终执行命令：

```bash
bun add typescript --clean
```

### 管理语言版本

```bash
siiway version [language] <version> [-- options...]
```

查看可用的语言版本配置：

```bash
siiway version --list
siiway version -l
```

配置路径（与 language 相同的分层方式）：

- `languages.<language>.version.backend`
- `languages.<language>.version.use`

内置后端：

- `uv`（默认模板：`uv python pin {version}`）
- `nvm`（默认模板：`nvm install {version} && nvm use {version}`）

也可通过 `languages.<language>.version.use` 自定义命令模板。

版本占位符：

- `{version}`
- `{options}`

示例配置：

```yaml
language: node
languages:
  python:
    version:
      backend: uv
  node:
    version:
      backend: nvm
```

示例：

```bash
siiway version 20
```

带模板替换参数：

```bash
siiway new python-service@latest my-python-service \
  --pm uv \
  --project-version 0.1.0 \
  --project-author "Your Name" \
  --project-description "My project"
```

创建完成后，CLI 会在生成项目的根目录中查找所有以 `.template` 结尾的文件，执行内置占位符替换，然后将文件名去掉 `.template` 后缀。

内置占位符：

- `{pm}`
- `{sw-version}`
- `{project-name}`
- `{project-version}`
- `{project-author}`
- `{project-description}`

## 认证

GitHub Token 优先级（从高到低）：

1. --token 参数
2. GITHUB_TOKEN 环境变量
3. CLI 配置中的 github_token

示例：

```bash
siiway new python-service@latest my-service --token <your_token>
```

## 配置命令

查看当前配置：

```bash
siiway config show
```

按键名设置配置：

```bash
siiway config set <key_name> <value>
```

重置为默认配置：

```bash
siiway config reset
```

### 支持的 key

- token（github_token 的别名）
- github_token
- current_registry
- registries.<alias>.owner
- registries.<alias>.repo
- registries.<alias>.ref
- registries.<alias>.path
- registries.<alias>.source（格式：owner/repo）

### 注册中心配置示例

创建或更新名为 internal 的注册中心别名：

```bash
siiway config set registries.internal.source your-org/cli-templates
siiway config set registries.internal.ref main
siiway config set registries.internal.path templates.yaml
siiway config set current_registry internal
```

设置默认 GitHub Token：

```bash
siiway config set github_token <your_token>
```

## 模板元数据（templates.yaml）

一个最小模板定义示例：

```yaml
- name: python-service
  description: Python starter project
  repo_url: https://github.com/your-org/python-template
  branch: main
  path: template
```

### 项目名替换规则

你可以定义在克隆后执行的正则替换规则。

方案 A：简写（replace.project_name）

```yaml
- name: python-service
  repo_url: https://github.com/your-org/python-template
  replace:
    project_name: pyproject.toml:siiway-python-template
```

含义：

- 文件：pyproject.toml
- 正则模式：siiway-python-template
- 替换为：新项目名

方案 B：显式规则（project_name_regex_rules）

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

规则字段说明：

- file：匹配单个文件（相对生成后的项目目录）
- file_pattern：使用 glob 匹配多个文件
- pattern：正则表达式
- replacement：替换文本（支持 {{project_name}}）

## CI 构建产物

GitHub Actions 工作流会构建以下平台：

- linux/amd64
- linux/arm64
- darwin/amd64
- darwin/arm64
- windows/amd64
- windows/arm64

工作流会按平台分别上传 artifact。

## Release 发布

GitHub Release 工作流文件：`.github/workflows/release.yml`

- 推送 `v*` 标签（例如 `v1.0.0`）后，会自动构建全部平台并发布 Release 资源。
- 也支持手动触发工作流并传入 `tag` 参数。

## 常见问题

- 如果拉取模板元数据失败，请检查注册中心配置：
  - owner、repo、ref、path
- 如果克隆私有仓库失败：
  - 请提供具备仓库访问权限的有效 Token。
- 如果 v* 版本未匹配到分支：
  - 请确认远程仓库中存在对应分支。

## License

本项目采用 MIT License。详见 [LICENSE](LICENSE)。
