// Command dev provides portable development tasks using only Go's standard library.
package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const appName = "go-cli-starter"

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "Usage: dev <build|fmt|fmt-check|doctor|doctor-agent|template-check|mcp-check>")
		os.Exit(2)
	}
	if err := run(os.Args[1]); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}

func run(task string) error {
	switch task {
	case "build":
		return build()
	case "fmt":
		return format(true)
	case "fmt-check":
		return format(false)
	case "doctor":
		return doctor(false)
	case "doctor-agent":
		return doctor(true)
	case "template-check":
		return verifyTemplate()
	case "mcp-check":
		return checkMCP()
	default:
		return fmt.Errorf("unknown task %q", task)
	}
}

func command(dir, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s %s: %w", name, strings.Join(args, " "), err)
	}
	return nil
}

func build() error {
	if err := os.MkdirAll("bin", 0o755); err != nil {
		return err
	}
	name := appName
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	version, commit := "dev", "none"
	if data, err := exec.Command("git", "describe", "--tags", "--always", "--dirty").Output(); err == nil {
		version = strings.TrimSpace(string(data))
	}
	if data, err := exec.Command("git", "rev-parse", "--short", "HEAD").Output(); err == nil {
		commit = strings.TrimSpace(string(data))
	}
	flags := fmt.Sprintf("-s -w -X main.version=%s -X main.commit=%s -X main.date=%s", version, commit, time.Now().UTC().Format(time.RFC3339))
	return command("", "go", "build", "-trimpath", "-ldflags", flags, "-o", filepath.Join("bin", name), ".")
}

func format(write bool) error {
	var files []string
	err := filepath.WalkDir(".", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			switch entry.Name() {
			case ".git", ".idea", ".superpowers", "bin", "dist", "vendor":
				return filepath.SkipDir
			}
		} else if strings.HasSuffix(path, ".go") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return nil
	}
	flag := "-l"
	if write {
		flag = "-w"
	}
	cmd := exec.Command("gofmt", append([]string{flag}, files...)...)
	data, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("gofmt: %w\n%s", err, data)
	}
	if !write && len(data) != 0 {
		return fmt.Errorf("run mise run fmt to format:\n%s", data)
	}
	return nil
}

func doctor(agent bool) error {
	tools := []struct {
		name, version string
		args          []string
	}{
		{"go", "go1.27.3", []string{"version"}},
		{"gopls", "v0.24.0", []string{"version"}},
		{"golangci-lint", "2.15.0", []string{"version"}},
		{"goreleaser", "2.19.0", []string{"--version"}},
	}
	var problems []error
	if _, err := exec.LookPath("git"); err != nil {
		problems = append(problems, errors.New("git is missing"))
	} else {
		fmt.Println("OK git")
	}
	for _, tool := range tools {
		data, err := exec.Command(tool.name, tool.args...).CombinedOutput()
		if err != nil {
			problems = append(problems, fmt.Errorf("%s unavailable: %w", tool.name, err))
			continue
		}
		if !strings.Contains(string(data), tool.version) {
			problems = append(problems, fmt.Errorf("%s does not match pinned version %s; run mise run setup", tool.name, tool.version))
			continue
		}
		fmt.Printf("OK %s %s\n", tool.name, tool.version)
	}
	if agent {
		for _, name := range []string{"codex", "rtk"} {
			if _, err := exec.LookPath(name); err != nil {
				fmt.Printf("OPTIONAL %s is not on PATH\n", name)
			} else {
				data, err := exec.Command(name, "--version").CombinedOutput()
				if err != nil {
					fmt.Printf("OPTIONAL %s version unavailable: %v\n", name, err)
				} else {
					fmt.Printf("USER %s", data)
					if len(data) == 0 || data[len(data)-1] != '\n' {
						fmt.Println()
					}
				}
			}
		}
		fmt.Println("USER superpowers: confirm the user-level skills are available in Codex; no installation path is assumed.")
		fmt.Println("Run mise run mcp:check for a live gopls connection check.")
	}
	return errors.Join(problems...)
}
