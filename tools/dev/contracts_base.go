package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

const initialContractFile = "test/testdata/contracts/initial.json"
const legacyContractFile = "testdata/contracts/v1.json"

// readContractBase 只在两个历史路径均不存在时使用首次基线。
func readContractBase(ctx context.Context, ref string) ([]byte, bool, error) {
	if ref == "" || strings.HasPrefix(ref, "-") {
		return nil, false, fmt.Errorf("无效的 Git 基准引用")
	}
	resolved, err := exec.CommandContext(ctx, "git", "rev-parse", "--verify", "--end-of-options", ref+"^{commit}").Output()
	if err != nil {
		return nil, false, fmt.Errorf("无法解析 Git 基准：%w", err)
	}
	commit := strings.TrimSpace(string(resolved))
	entries, err := exec.CommandContext(ctx, "git", "ls-tree", "-z", commit, "--", contractFile, legacyContractFile).Output()
	if err != nil {
		return nil, false, fmt.Errorf("读取 Git 基准文件清单失败：%w", err)
	}
	modes := map[string]string{}
	for _, entry := range bytes.Split(entries, []byte{0}) {
		if len(entry) == 0 {
			continue
		}
		pair := strings.SplitN(string(entry), "\t", 2)
		if len(pair) != 2 {
			return nil, false, fmt.Errorf("基准 Git 文件记录无效")
		}
		meta := strings.Fields(pair[0])
		if len(meta) != 3 {
			return nil, false, fmt.Errorf("基准 Git 文件类型无效")
		}
		modes[pair[1]] = meta[0] + " " + meta[1]
	}
	for _, file := range []string{contractFile, legacyContractFile} {
		mode, exists := modes[file]
		if !exists {
			continue
		}
		if mode != "100644 blob" && mode != "100755 blob" {
			return nil, false, fmt.Errorf("基准 Git 快照不是普通文件：%s", file)
		}
		data, err := exec.CommandContext(ctx, "git", "show", commit+":"+file).Output()
		if err != nil {
			return nil, false, fmt.Errorf("读取 Git 基准快照失败：%w", err)
		}
		return data, false, nil
	}
	data, err := os.ReadFile(initialContractFile)
	return data, true, err
}
