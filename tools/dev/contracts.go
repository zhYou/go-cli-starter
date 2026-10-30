package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/zhYou/go-cli-starter/internal/app/cli"
	"github.com/zhYou/go-cli-starter/internal/base/paths"
	"github.com/zhYou/go-cli-starter/internal/dev/contract"
)

const contractFile = "test/testdata/contracts/v1.json"

func captureContract(ctx context.Context) ([]byte, error) {
	dir, err := os.MkdirTemp("", "cli-contract-*")
	if err != nil {
		return nil, err
	}
	defer func() { _ = os.RemoveAll(dir) }()
	results := map[string]any{}
	for key, args := range map[string][]string{schemaCommand: {schemaCommand}, greetCommand: {greetCommand, nameFlag, "Codex", jsonFlag}, versionCommand: {versionCommand, jsonFlag}} {
		var out, errOut bytes.Buffer
		code := cli.Run(ctx, args, cli.Options{Name: "contract-cli", Version: "1.0.0", Commit: "abc", Date: "2000-01-01", Out: &out, ErrOut: &errOut, ConfigDir: dir, Directories: &paths.Directories{State: filepath.Join(dir, "state")}, LookupEnv: func(string) (string, bool) { return "", false }})
		if code != 0 {
			return nil, fmt.Errorf("捕获 %s 失败：%s", key, &errOut)
		}
		var value any
		if err := json.Unmarshal(out.Bytes(), &value); err != nil {
			return nil, err
		}
		if key == schemaCommand {
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
		//nolint:gosec // G304：开发任务只读取调用者指定的项目目录或本地构建 fixture，不作为服务接收外部路径。
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		if against != "" {
			//nolint:gosec // G204：开发工具读取显式项目文件或执行固定 Git 子命令，不经过 shell，不是外部服务入口。
			old, err := exec.Command(gitProgram, "show", against+":"+filepath.ToSlash(path)).Output()
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
		base, initial, err = readContractBase(ctx, against)
		if err != nil {
			return err
		}
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
	//nolint:gosec // G301：这是公开构建产物或机器契约文件的既定权限，不包含凭据。
	if err := os.MkdirAll(filepath.Dir(contractFile), 0755); err != nil {
		return err
	}
	//nolint:gosec // G306：这是公开构建产物或机器契约文件的既定权限，不包含凭据。
	return os.WriteFile(contractFile, append(data, '\n'), 0644)
}
