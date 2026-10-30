package command

import (
	"context"
	"os/exec"
)

// Windows 暂用标准库后端；原生 Job 后端将替换此函数，公共层不依赖 exec.Cmd 内部。
func runPlatform(ctx context.Context, program string, args []string, options Execution) error {
	// #nosec G204 -- 程序及 argv 由显式调用提供，不经 shell。
	cmd := exec.CommandContext(ctx, program, args...)
	cmd.Dir, cmd.Env = options.Dir, options.Env
	cmd.Stdin, cmd.Stdout, cmd.Stderr = options.IO.In, options.IO.Out, options.IO.ErrOut
	cmd.WaitDelay = options.WaitDelay
	return cmd.Run()
}
