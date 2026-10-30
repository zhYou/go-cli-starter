package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/zhYou/go-cli-starter/internal/dev/scaffold"
)

// checkMCP 验证 Codex 配置中使用的 stdio 命令。
func checkMCP() error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "mise", "exec", "--", goplsProgram, "mcp")
	cmd.Stderr = os.Stderr
	in, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	out, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	defer func() { _ = in.Close(); _ = cmd.Process.Kill(); _ = cmd.Wait() }()
	encoder, decoder := json.NewEncoder(in), json.NewDecoder(out)
	call := func(id int, method string, params any) (json.RawMessage, error) {
		if err := encoder.Encode(map[string]any{"jsonrpc": "2.0", "id": id, "method": method, "params": params}); err != nil {
			return nil, err
		}
		for {
			var response struct {
				ID     int             `json:"id"`
				Result json.RawMessage `json:"result"`
				Error  json.RawMessage `json:"error"`
			}
			if err := decoder.Decode(&response); err != nil {
				return nil, fmt.Errorf("读取 %s 响应失败：%w", method, err)
			}
			if response.ID != id {
				continue
			}
			if len(response.Error) != 0 {
				return nil, fmt.Errorf("%s 调用失败：%s", method, response.Error)
			}
			return response.Result, nil
		}
	}
	if _, err := call(1, "initialize", map[string]any{
		"protocolVersion": "2025-03-26", "capabilities": map[string]any{},
		"clientInfo": map[string]string{"name": "template-check", mcpVersionField: "1"},
	}); err != nil {
		return err
	}
	if err := encoder.Encode(map[string]any{"jsonrpc": "2.0", "method": "notifications/initialized"}); err != nil {
		return err
	}
	result, err := call(2, "tools/list", map[string]any{})
	if err != nil {
		return err
	}
	var catalog struct {
		Tools []struct {
			Name string `json:"name"`
		} `json:"tools"`
	}
	if err := json.Unmarshal(result, &catalog); err != nil {
		return err
	}
	found := false
	for _, tool := range catalog.Tools {
		if tool.Name == "go_diagnostics" {
			found = true
		}
	}
	if !found {
		return errors.New("gopls 未提供 go_diagnostics 工具")
	}
	project, err := scaffold.Inspect(".")
	if err != nil {
		return err
	}
	path, err := filepath.Abs(filepath.Join(project.CommandPath, "main.go"))
	if err != nil {
		return err
	}
	result, err = call(3, "tools/call", map[string]any{"name": "go_diagnostics", "arguments": map[string]any{"files": []string{path}}})
	if err != nil {
		return err
	}
	var diagnostic struct {
		IsError bool `json:"isError"`
	}
	if err := json.Unmarshal(result, &diagnostic); err != nil {
		return err
	}
	if diagnostic.IsError {
		return fmt.Errorf("gopls 诊断失败：%s", result)
	}
	fmt.Printf("MCP 已初始化，发现 %d 个工具，已诊断 %s。\n", len(catalog.Tools), path)
	return nil
}
