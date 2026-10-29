// Command init 重命名模板的新副本。
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/zhYou/go-cli-starter/internal/scaffold"
)

func main() {
	root, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, "错误：获取当前目录失败：", err)
		os.Exit(1)
	}
	os.Exit(run(os.Args[1:], root, os.Stdout, os.Stderr))
}

func run(args []string, root string, out, errOut io.Writer) int {
	flags := flag.NewFlagSet("init", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	module := flags.String("module", "", "Go 模块路径，例如 github.com/acme/mycli")
	name := flags.String("name", "", "可执行文件名称，例如 mycli")
	usage := func() {
		_, _ = fmt.Fprintln(out, "用法：mise run init --module 模块路径 --name 名称")
		flags.VisitAll(func(f *flag.Flag) { _, _ = fmt.Fprintf(out, "  --%s <string>\t%s\n", f.Name, f.Usage) })
	}
	flags.Usage = func() {}
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			usage()
			return 0
		}
		_, _ = fmt.Fprintln(errOut, "错误：解析参数失败：", err)
		return 2
	}
	if flags.NArg() != 0 {
		_, _ = fmt.Fprintln(errOut, "错误：不接受位置参数")
		return 2
	}
	changed, err := scaffold.Initialize(root, *module, *name)
	if err != nil {
		_, _ = fmt.Fprintln(errOut, "错误：", err)
		return 1
	}
	_, _ = fmt.Fprintf(out, "已初始化 %s（%s），更新了 %d 个文件。\n", *module, *name, len(changed))
	return 0
}
