package toolchain

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// matchInstalledPackage 核对 mise 已安装包的本地元数据，不执行包管理器或访问网络。
func matchInstalledPackage(key, prefix, want string) error {
	if !filepath.IsAbs(prefix) {
		return fmt.Errorf("工具 %s 的安装位置不是绝对路径", key)
	}
	kind, name, _ := strings.Cut(key, ":")
	var version string
	var err error
	if kind == "npm" {
		version, err = npmVersion(prefix, name)
	} else {
		version, err = pythonPackageVersion(prefix, name)
	}
	if err != nil {
		return fmt.Errorf("核对 %s 本地元数据: %w", key, err)
	}
	if version != want {
		return fmt.Errorf("%s 与固定版本 %s 不符，请运行 mise run setup", key, want)
	}
	return nil
}

func npmVersion(prefix, name string) (string, error) {
	for _, base := range []string{prefix, filepath.Join(prefix, "lib")} {
		path := filepath.Join(base, "node_modules", filepath.FromSlash(name), "package.json")
		//nolint:gosec // G304：路径来自固定 mise 工具的已安装目录和登记包名，只读取版本元数据。
		data, err := os.ReadFile(path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return "", err
		}
		var metadata struct {
			Name    string `json:"name"`
			Version string `json:"version"`
		}
		if err := json.Unmarshal(data, &metadata); err != nil {
			return "", err
		}
		if metadata.Name != name || metadata.Version == "" {
			return "", fmt.Errorf("包名或版本无效: %s", path)
		}
		return metadata.Version, nil
	}
	return "", fmt.Errorf("未找到 %s 的 package.json", name)
}

func pythonPackageVersion(prefix, name string) (string, error) {
	normalized := strings.ReplaceAll(name, "-", "_")
	for _, base := range []string{prefix, filepath.Join(prefix, name)} {
		for _, library := range []string{"lib/python*/site-packages", "Lib/site-packages"} {
			matches, err := filepath.Glob(filepath.Join(base, filepath.FromSlash(library), normalized+"-*.dist-info", "METADATA"))
			if err != nil {
				return "", err
			}
			if len(matches) == 0 {
				continue
			}
			if len(matches) != 1 {
				return "", fmt.Errorf("%s 存在多份版本元数据", name)
			}
			data, err := os.ReadFile(matches[0])
			if err != nil {
				return "", err
			}
			var found, version string
			for line := range strings.SplitSeq(string(data), "\n") {
				if line == "" {
					break
				}
				if value, ok := strings.CutPrefix(line, "Name: "); ok {
					found = strings.TrimSpace(value)
				}
				if value, ok := strings.CutPrefix(line, "Version: "); ok {
					version = strings.TrimSpace(value)
				}
			}
			if strings.ReplaceAll(found, "-", "_") != normalized || version == "" {
				return "", fmt.Errorf("包名或版本无效: %s", matches[0])
			}
			return version, nil
		}
	}
	return "", fmt.Errorf("未找到 %s 的 METADATA", name)
}
