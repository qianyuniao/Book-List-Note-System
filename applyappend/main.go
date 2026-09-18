package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// apply 读取批次文件（格式：===FILE: 相对路径 分隔），把正文追加到对应笔记末尾
// 用法: go run .\applyappend batch01.md
func main() {
	if len(os.Args) < 2 {
		fmt.Println("usage: applyappend <batchfile>")
		os.Exit(1)
	}
	data, err := os.ReadFile(os.Args[1])
	if err != nil {
		fmt.Println("read batch:", err)
		os.Exit(1)
	}
	parts := strings.Split(string(data), "===FILE:")
	applied, skipped := 0, 0
	for _, p := range parts[1:] {
		lines := strings.SplitN(strings.TrimPrefix(p, "\n"), "\n", 2)
		path := strings.TrimSpace(lines[0])
		body := ""
		if len(lines) > 1 {
			body = lines[1]
		}
		if path == "" || body == "" {
			continue
		}
		if _, e := os.Stat(path); e != nil {
			fmt.Println("MISSING:", path)
			skipped++
			continue
		}
		f, e := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0644)
		if e != nil {
			fmt.Println("OPEN ERR:", path, e)
			skipped++
			continue
		}
		// 确保以空行分隔原内容与新正文
		r := bufio.NewReader(strings.NewReader(body))
		first, _ := r.ReadString('\n')
		if !strings.HasPrefix(first, "### ") {
			f.WriteString("\n")
		}
		f.WriteString(body)
		if !strings.HasSuffix(body, "\n") {
			f.WriteString("\n")
		}
		f.Close()
		applied++
		fmt.Println("OK:", path)
	}
	fmt.Printf("applied=%d skipped=%d\n", applied, skipped)
}
