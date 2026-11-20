package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"sort"
)

const openCodeConfigPath = ".opencode/opencode.jsonc"

const (
	openCodeActionShell = "shell"
	openCodeEffectAllow = "allow"
	openCodeEffectAsk   = "ask"
	openCodeEffectDeny  = "deny"
)

type openCodeConfig struct {
	Schema      string               `json:"$schema"`
	Worktree    openCodeWorktree     `json:"worktree"`
	MCP         openCodeMCP          `json:"mcp"`
	Permissions []openCodePermission `json:"permissions"`
}

type openCodeWorktree struct {
	Directory string `json:"directory"`
}

type openCodeMCP struct {
	Timeout openCodeMCPTimeout           `json:"timeout"`
	Servers map[string]openCodeMCPServer `json:"servers"`
}

type openCodeMCPTimeout struct {
	Startup   int `json:"startup"`
	Execution int `json:"execution"`
}

type openCodeMCPServer struct {
	Type      string          `json:"type"`
	Command   []string        `json:"command"`
	Directory string          `json:"cwd"`
	Disabled  bool            `json:"disabled"`
	Pool      bool            `json:"pool"`
	CodeMode  *bool           `json:"codemode"`
	Timeout   json.RawMessage `json:"timeout"`
}

type openCodePermission struct {
	Action   string `json:"action"`
	Resource string `json:"resource"`
	Effect   string `json:"effect"`
}

// checkOpenCodeConfig 校验项目级 OpenCode 适配层的固定边界，避免权限或 MCP 配置静默漂移。
func checkOpenCodeConfig(root string) error {
	root, err := filepath.Abs(root)
	if err != nil {
		return fmt.Errorf(resolveRepositoryError, err)
	}
	path := filepath.Join(root, filepath.FromSlash(openCodeConfigPath))
	info, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("检查 %s: %w", path, err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("%s 不是普通文件", path)
	}
	config, err := loadOpenCodeConfig(path)
	if err != nil {
		return err
	}
	return validateOpenCodeConfig(path, config)
}

func loadOpenCodeConfig(path string) (openCodeConfig, error) {
	//nolint:gosec // G304：只读取调用者指定 checkout 的固定项目配置。
	data, err := os.ReadFile(path)
	if err != nil {
		return openCodeConfig{}, fmt.Errorf(readPathError, path, err)
	}
	data, err = normalizeJSONC(data)
	if err != nil {
		return openCodeConfig{}, fmt.Errorf("%s 的 JSONC 无效: %w", path, err)
	}
	var config openCodeConfig
	decoder := json.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(&config); err != nil {
		return openCodeConfig{}, fmt.Errorf("%s 的 JSONC 无效: %w", path, err)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			err = errors.New("存在多余 JSON 文档")
		}
		return openCodeConfig{}, fmt.Errorf("%s 存在多余内容: %w", path, err)
	}
	return config, nil
}

// normalizeJSONC 将注释和尾逗号替换为空白，保留原始行结构，再交给 encoding/json 严格校验。
func normalizeJSONC(data []byte) ([]byte, error) {
	normalized := bytes.Clone(data)
	inString := false
	escaped := false
	for i := 0; i < len(normalized); i++ {
		switch {
		case inString && escaped:
			escaped = false
		case inString && normalized[i] == '\\':
			escaped = true
		case inString && normalized[i] == '"':
			inString = false
		case inString:
			continue
		case normalized[i] == '"':
			inString = true
		case normalized[i] == '/' && i+1 < len(normalized) && normalized[i+1] == '/':
			end := bytes.IndexByte(normalized[i+2:], '\n')
			if end < 0 {
				end = len(normalized)
			} else {
				end += i + 2
			}
			blankJSONCRange(normalized, i, end)
			i = end - 1
		case normalized[i] == '/' && i+1 < len(normalized) && normalized[i+1] == '*':
			end := bytes.Index(normalized[i+2:], []byte("*/"))
			if end < 0 {
				return nil, errors.New("块注释未闭合")
			}
			end += i + 4
			blankJSONCRange(normalized, i, end)
			i = end - 1
		}
	}
	return removeJSONCTrailingCommas(normalized), nil
}

// blankJSONCRange 用空格覆盖注释，保留换行以便 JSON 错误继续指向原始行。
func blankJSONCRange(data []byte, start, end int) {
	for i := start; i < end; i++ {
		if data[i] != '\n' && data[i] != '\r' {
			data[i] = ' '
		}
	}
}

// removeJSONCTrailingCommas 删除对象或数组末尾多余的逗号，允许 OpenCode 支持的标准 JSONC。
func removeJSONCTrailingCommas(data []byte) []byte {
	inString := false
	escaped := false
	for i := range len(data) {
		switch {
		case inString && escaped:
			escaped = false
		case inString && data[i] == '\\':
			escaped = true
		case inString && data[i] == '"':
			inString = false
		case inString:
			continue
		case data[i] == '"':
			inString = true
		case data[i] == ',':
			next := i + 1
			for next < len(data) && isJSONCWhitespace(data[next]) {
				next++
			}
			if next < len(data) && (data[next] == '}' || data[next] == ']') {
				data[i] = ' '
			}
		}
	}
	return data
}

func isJSONCWhitespace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r'
}

func validateOpenCodeConfig(path string, config openCodeConfig) error {
	if config.Schema != "https://opencode.ai/config.json" {
		return fmt.Errorf("%s 的 $schema 不一致", path)
	}
	if config.Worktree.Directory != ".worktrees" {
		return fmt.Errorf("%s 的 worktree.directory 必须为 .worktrees", path)
	}
	if config.MCP.Timeout.Startup != 300000 || config.MCP.Timeout.Execution != 180000 {
		return fmt.Errorf("%s 的 MCP 启动或执行超时不一致", path)
	}
	expectedServers := map[string][]string{
		"serena":   {"mise", "run", "--quiet", "mcp:serve:opencode"},
		"graphify": {"mise", "run", "--quiet", "graphify:serve"},
	}
	for name, command := range expectedServers {
		server, ok := config.MCP.Servers[name]
		if !ok {
			return fmt.Errorf("%s 缺少 MCP 服务 %s", path, name)
		}
		if err := validateOpenCodeServer(path, name, server, command); err != nil {
			return err
		}
	}
	var unknownServers []string
	for name := range config.MCP.Servers {
		if _, ok := expectedServers[name]; !ok {
			unknownServers = append(unknownServers, name)
		}
	}
	if len(unknownServers) != 0 {
		sort.Strings(unknownServers)
		return fmt.Errorf("%s 包含未登记的 MCP 服务 %v", path, unknownServers)
	}
	if !slices.Equal(config.Permissions, expectedOpenCodePermissions()) {
		return fmt.Errorf("%s 的 permissions 与受检边界不一致", path)
	}
	return nil
}

func validateOpenCodeServer(path, name string, server openCodeMCPServer, command []string) error {
	if server.Type != "local" {
		return fmt.Errorf("%s 的 MCP 服务 %s 必须使用 local", path, name)
	}
	if !slices.Equal(server.Command, command) {
		return fmt.Errorf("%s 的 MCP 服务 %s 启动命令不一致", path, name)
	}
	if server.Directory != "." {
		return fmt.Errorf("%s 的 MCP 服务 %s 必须从 workspace 启动", path, name)
	}
	if server.Disabled || server.Pool || server.CodeMode != nil || len(server.Timeout) != 0 {
		return fmt.Errorf("%s 的 MCP 服务 %s 不得禁用、共享连接或覆盖全局超时和 Code Mode", path, name)
	}
	return nil
}

func expectedOpenCodePermissions() []openCodePermission {
	return []openCodePermission{
		{Action: openCodeActionShell, Resource: "*", Effect: openCodeEffectAsk},
		{Action: openCodeActionShell, Resource: "mise run *", Effect: openCodeEffectAllow},
		{Action: openCodeActionShell, Resource: "mise exec -- *", Effect: openCodeEffectAllow},
		{Action: openCodeActionShell, Resource: "go test *", Effect: openCodeEffectAllow},
		{Action: openCodeActionShell, Resource: "go vet *", Effect: openCodeEffectAllow},
		{Action: openCodeActionShell, Resource: "go build *", Effect: openCodeEffectAllow},
		{Action: openCodeActionShell, Resource: "go install *", Effect: openCodeEffectAllow},
		{Action: openCodeActionShell, Resource: "go list *", Effect: openCodeEffectAllow},
		{Action: openCodeActionShell, Resource: "go version *", Effect: openCodeEffectAllow},
		{Action: openCodeActionShell, Resource: "go mod tidy -diff *", Effect: openCodeEffectAllow},
		{Action: openCodeActionShell, Resource: "golangci-lint fmt *", Effect: openCodeEffectAllow},
		{Action: openCodeActionShell, Resource: "golangci-lint run *", Effect: openCodeEffectAllow},
		{Action: openCodeActionShell, Resource: "gopls *", Effect: openCodeEffectAllow},
		{Action: openCodeActionShell, Resource: "actionlint *", Effect: openCodeEffectAllow},
		{Action: openCodeActionShell, Resource: "govulncheck *", Effect: openCodeEffectAllow},
		{Action: openCodeActionShell, Resource: "goreleaser check *", Effect: openCodeEffectAllow},
		{Action: openCodeActionShell, Resource: "goreleaser release --snapshot *", Effect: openCodeEffectAllow},
		{Action: openCodeActionShell, Resource: "taplo *", Effect: openCodeEffectAllow},
		{Action: openCodeActionShell, Resource: "gh pr view *", Effect: openCodeEffectAllow},
		{Action: openCodeActionShell, Resource: "gh pr list *", Effect: openCodeEffectAllow},
		{Action: openCodeActionShell, Resource: "gh pr checks *", Effect: openCodeEffectAllow},
		{Action: openCodeActionShell, Resource: "gh pr diff *", Effect: openCodeEffectAllow},
		{Action: openCodeActionShell, Resource: "gh pr status *", Effect: openCodeEffectAllow},
		{Action: openCodeActionShell, Resource: "gh issue view *", Effect: openCodeEffectAllow},
		{Action: openCodeActionShell, Resource: "gh issue list *", Effect: openCodeEffectAllow},
		{Action: openCodeActionShell, Resource: "gh repo view *", Effect: openCodeEffectAllow},
		{Action: openCodeActionShell, Resource: "gh label list *", Effect: openCodeEffectAllow},
		{Action: openCodeActionShell, Resource: "gh run list *", Effect: openCodeEffectAllow},
		{Action: openCodeActionShell, Resource: "gh event listen *", Effect: openCodeEffectAllow},
		{Action: "graphify_*", Resource: "*", Effect: openCodeEffectDeny},
		{Action: "graphify_query_graph", Resource: "*", Effect: openCodeEffectAllow},
		{Action: "graphify_get_node", Resource: "*", Effect: openCodeEffectAllow},
		{Action: "graphify_get_neighbors", Resource: "*", Effect: openCodeEffectAllow},
		{Action: "graphify_get_community", Resource: "*", Effect: openCodeEffectAllow},
		{Action: "graphify_god_nodes", Resource: "*", Effect: openCodeEffectAllow},
		{Action: "graphify_graph_stats", Resource: "*", Effect: openCodeEffectAllow},
		{Action: "graphify_shortest_path", Resource: "*", Effect: openCodeEffectAllow},
	}
}
