package main

import (
	"strings"
	"testing"
)

func TestUnknownTaskUsesChinese(t *testing.T) {
	err := run("unknown")
	if err == nil || !strings.Contains(err.Error(), "未知任务") || !strings.Contains(err.Error(), "unknown") {
		t.Fatalf("未知任务错误=%v", err)
	}
}
