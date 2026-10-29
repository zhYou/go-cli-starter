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
)

// checkMCP exercises the same stdio command configured for Codex.
func checkMCP() error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "mise", "exec", "--", "gopls", "mcp")
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
				return nil, fmt.Errorf("%s: %w", method, err)
			}
			if response.ID != id {
				continue
			}
			if len(response.Error) != 0 {
				return nil, fmt.Errorf("%s: %s", method, response.Error)
			}
			return response.Result, nil
		}
	}
	if _, err := call(1, "initialize", map[string]any{
		"protocolVersion": "2025-03-26", "capabilities": map[string]any{},
		"clientInfo": map[string]string{"name": "template-check", "version": "1"},
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
		return errors.New("gopls did not expose go_diagnostics")
	}
	path, err := filepath.Abs("main.go")
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
		return fmt.Errorf("gopls diagnostics failed: %s", result)
	}
	fmt.Printf("MCP initialized; discovered %d tools; diagnosed %s.\n", len(catalog.Tools), path)
	return nil
}
