package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

func releaseFile(dist, name string) (string, error) {
	if name == "" || filepath.Base(name) != name || !regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`).MatchString(name) || name == ".." {
		return "", fmt.Errorf("不安全的发布路径 %q", name)
	}
	path := filepath.Join(dist, name)
	info, err := os.Lstat(path)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("发布产物必须为常规文件：%s", name)
	}
	return path, nil
}
func checkReleaseSnapshot(dist string) error {
	path, err := releaseFile(dist, "checksums.txt")
	if err != nil {
		return err
	}
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()
	checksums := map[string]string{}
	scanner := bufio.NewScanner(file)
	linePattern := regexp.MustCompile(`^([0-9a-f]{64}) [ *]([^\r\n]+)$`)
	for scanner.Scan() {
		line := scanner.Text()
		match := linePattern.FindStringSubmatch(line)
		if len(match) != 3 {
			return fmt.Errorf("无效的 SHA-256 校验行：%q", line)
		}
		name := match[2]
		if _, ok := checksums[name]; ok {
			return fmt.Errorf("校验清单包含重复路径：%s", name)
		}
		path, err := releaseFile(dist, name)
		if err != nil {
			return err
		}
		input, err := os.Open(path)
		if err != nil {
			return err
		}
		hash := sha256.New()
		_, copyErr := io.Copy(hash, input)
		closeErr := input.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
		if hex.EncodeToString(hash.Sum(nil)) != match[1] {
			return fmt.Errorf("发布产物摘要不符：%s", name)
		}
		checksums[name] = match[1]
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	pattern := regexp.MustCompile("^" + regexp.QuoteMeta(appName) + `_(.+)_(linux|darwin|windows)_(amd64|arm64)\.(tar\.gz|zip)$`)
	targets := map[string]bool{}
	version := ""
	for name := range checksums {
		match := pattern.FindStringSubmatch(name)
		if len(match) == 0 {
			continue
		}
		target := match[2] + "_" + match[3]
		if targets[target] {
			return fmt.Errorf("目标归档重复：%s", target)
		}
		targets[target] = true
		if version == "" {
			version = match[1]
		} else if version != match[1] {
			return fmt.Errorf("发布归档版本不一致")
		}
		if match[2] == "windows" && match[4] != "zip" || match[2] != "windows" && match[4] != "tar.gz" {
			return fmt.Errorf("归档格式与平台不符：%s", name)
		}
		sbom := name + ".spdx.json"
		if _, ok := checksums[sbom]; !ok {
			return fmt.Errorf("归档缺少已校验 SBOM：%s", name)
		}
		if err := validateSPDX(filepath.Join(dist, sbom), name); err != nil {
			return err
		}
	}
	if len(targets) != 6 {
		return fmt.Errorf("必须包含六目标归档及其 SBOM，实际 %d 个", len(targets))
	}
	return nil
}
func validateSPDX(path, archive string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var doc struct {
		Version   string `json:"spdxVersion"`
		ID        string `json:"SPDXID"`
		Name      string `json:"name"`
		Namespace string `json:"documentNamespace"`
		Creation  struct {
			Creators []string `json:"creators"`
			Created  string   `json:"created"`
		} `json:"creationInfo"`
		Packages []struct {
			ID   string `json:"SPDXID"`
			Name string `json:"name"`
		} `json:"packages"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return err
	}
	uri, err := url.Parse(doc.Namespace)
	if err != nil || !uri.IsAbs() || !strings.HasPrefix(doc.Version, "SPDX-2.") || doc.ID != "SPDXRef-DOCUMENT" || doc.Name != archive || len(doc.Creation.Creators) == 0 || doc.Creation.Created == "" || len(doc.Packages) == 0 {
		return fmt.Errorf("归档 %s 的 SPDX 文档结构或身份无效", archive)
	}
	for _, p := range doc.Packages {
		if p.ID == "" || p.Name == "" {
			return fmt.Errorf("归档 %s 的 SPDX 包缺少标识", archive)
		}
	}
	return nil
}
