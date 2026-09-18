package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// scan 统计 books/ 下没有正文（无 "### " 划线章节）的笔记文件
func main() {
	var total, empty int
	var empties []string
	filepath.Walk("books", func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".md") {
			return nil
		}
		total++
		f, e := os.Open(path)
		if e != nil {
			return nil
		}
		defer f.Close()
		hasBody := false
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			if strings.HasPrefix(sc.Text(), "### ") {
				hasBody = true
				break
			}
		}
		if !hasBody {
			empty++
			empties = append(empties, path)
		}
		return nil
	})
	fmt.Printf("total=%d empty=%d\n", total, empty)
	os.WriteFile("empty_list.txt", []byte(strings.Join(empties, "\n")), 0644)
}
