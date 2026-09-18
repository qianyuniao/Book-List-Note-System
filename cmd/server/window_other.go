//go:build !windows

package main

import "fmt"

// winOp 窗口控制仅在 Windows 下有意义（直接调用 user32.dll）
func winOp(op string) error {
	return fmt.Errorf("窗口操作仅支持 Windows")
}
