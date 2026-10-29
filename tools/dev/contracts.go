package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/zhYou/go-cli-starter/internal/cli"
	"github.com/zhYou/go-cli-starter/internal/contract"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const contractFile = "testdata/contracts/v1.json"

func captureContract(ctx context.Context) ([]byte, error) {
	dir, err := os.MkdirTemp("", "cli-contract-*")
	if err != nil {
		return nil, err
	}
	defer func() { _ = os.RemoveAll(dir) }()
	results := map[string]any{}
	for key, args := range map[string][]string{"schema": {"schema"}, "greet": {"greet", "--name", "Codex", "--json"}, "version": {"version", "--json"}} {
		var out, errOut bytes.Buffer
		code := cli.Run(ctx, args, cli.Options{Name: "contract-cli", Version: "1.0.0", Commit: "abc", Date: "2000-01-01", Out: &out, ErrOut: &errOut, ConfigDir: dir, LookupEnv: func(string) (string, bool) { return "", false }})
		if code != 0 {
			return nil, fmt.Errorf("捕获 %s 失败：%s", key, &errOut)
		}
		var value any
		if err := json.Unmarshal(out.Bytes(), &value); err != nil {
			return nil, err
		}
		if key == "schema" {
			results[key] = value
		} else {
			if results["results"] == nil {
				results["results"] = map[string]any{}
			}
			results["results"].(map[string]any)[key] = shape(value)
		}
	}
	data, err := json.Marshal(results)
	if err != nil {
		return nil, err
	}
	return contract.Normalize(data)
}
func shape(value any) any {
	switch v := value.(type) {
	case map[string]any:
		out := map[string]any{}
		for k, item := range v {
			out[k] = shape(item)
		}
		return out
	case []any:
		return "array"
	case string:
		return "string"
	case float64:
		return "number"
	case bool:
		return "boolean"
	default:
		return "null"
	}
}
func validateContract(base, snapshot, actual []byte, records []contract.Record, initial bool) error {
	changes, err := contract.Diff(snapshot, actual)
	if err != nil {
		return err
	}
	if len(changes) > 0 {
		return fmt.Errorf("当前快照与程序不符，请审查后运行 mise run contract:update：%v", changes)
	}
	changes, err = contract.Diff(base, actual)
	if err != nil {
		return err
	}
	return contract.ValidateRecords(changes, records, initial)
}
func readRecords(against string) ([]contract.Record, error) {
	paths, err := filepath.Glob("docs/compatibility/changes/*.json")
	if err != nil {
		return nil, err
	}
	records := []contract.Record{}
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		if against != "" {
			old, err := exec.Command("git", "show", against+":"+filepath.ToSlash(path)).Output()
			if err == nil && bytes.Equal(old, data) {
				continue
			}
		}
		var record contract.Record
		decoder := json.NewDecoder(bytes.NewReader(data))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&record); err != nil {
			return nil, fmt.Errorf("读取 %s 失败：%w", path, err)
		}
		records = append(records, record)
	}
	return records, nil
}
func checkContract(ctx context.Context, against string) error {
	actual, err := captureContract(ctx)
	if err != nil {
		return err
	}
	snapshot, err := os.ReadFile(contractFile)
	if err != nil {
		return err
	}
	base := snapshot
	initial := false
	if against != "" {
		if strings.HasPrefix(against, "-") {
			return fmt.Errorf("无效的 Git 基准引用")
		}
		data, err := exec.CommandContext(ctx, "git", "show", against+":"+contractFile).Output()
		if err != nil {
			if _, refErr := exec.CommandContext(ctx, "git", "rev-parse", "--verify", against+"^{commit}").Output(); refErr != nil {
				return fmt.Errorf("无法解析 Git 基准：%w", refErr)
			}
			initial = true
			data, err = os.ReadFile("testdata/contracts/initial.json")
			if err != nil {
				return err
			}
		}
		base = data
	}
	records, err := readRecords(against)
	if err != nil {
		return err
	}
	return validateContract(base, snapshot, actual, records, initial)
}
func updateContract(ctx context.Context) error {
	data, err := captureContract(ctx)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(contractFile), 0755); err != nil {
		return err
	}
	return os.WriteFile(contractFile, append(data, '\n'), 0644)
}
