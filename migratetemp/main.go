// migratetemp 一次性整理工具：把 temp/ 下的扁平爬虫缓存按数据类型归档到子目录。
//
// 归档规则（与 cmd/scraper 的读写路径一一对应）：
//
//	booklists.json            → 保留在 temp/ 根（书单榜总览）
//	<bookId>-bookinfo.json    → temp/bookinfo/<bookId>.json（书籍元信息）
//	<bookId>-info.json        → temp/bookinfo/<bookId>.json（旧版爬虫同义命名）
//	<booklistId>.json         → temp/booklist/<id>.json（单书单详情，正文含 "booklist":）
//	<bookId>.json             → temp/bookmark/<bookId>.json（热门划线，正文含 "items":）
//	{"synckey":0}             → temp/bookmark/（无热门划线的空响应缓存）
//	含 "errcode" 的失效响应    → 直接删除（爬虫命中也会自行删除重抓）
//
// 由于书单 ID 与书籍 ID 都可能是纯数字或"数字_码"形态，无法靠文件名区分，
// 故对无后缀文件按响应内容首键判断类型；无法识别的文件保持原位并计入 unknown。
// 工具幂等：目标已存在或源文件缺失时自动跳过，可重复执行。
//
// 用法（项目根目录）：go run .\migratetemp
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	entries, err := os.ReadDir("temp")
	if err != nil {
		fmt.Fprintln(os.Stderr, "读取 temp/ 失败:", err)
		os.Exit(1)
	}
	counts := map[string]int{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			counts["skip"]++
			continue
		}
		name := e.Name()
		src := filepath.Join("temp", name)
		if name == "booklists.json" {
			counts["keep-root"]++
			continue
		}
		var sub string
		switch {
		case strings.HasSuffix(name, "-bookinfo.json"):
			sub = "bookinfo"
			name = strings.TrimSuffix(name, "-bookinfo.json") + ".json"
		case strings.HasSuffix(name, "-info.json"):
			sub = "bookinfo"
			name = strings.TrimSuffix(name, "-info.json") + ".json"
		default:
			head, err := os.ReadFile(src)
			if err != nil {
				counts["read-err"]++
				continue
			}
			body := strings.TrimSpace(string(head))
			if strings.Contains(body, `"errcode"`) {
				// 失效的错误响应缓存，无价值，删除
				if os.Remove(src) == nil {
					counts["dropped-err"]++
				}
				continue
			}
			n := len(body)
			if n > 256 {
				n = 256
			}
			prefix := body[:n]
			switch {
			case body == `{"synckey":0}`:
				sub = "bookmark" // 无划线书籍的空响应缓存
			case strings.Contains(prefix, `"booklist":`):
				sub = "booklist"
			case strings.Contains(prefix, `"items":`):
				sub = "bookmark"
			default:
				counts["unknown"]++
				continue
			}
		}
		if sub == "bookinfo" {
			// 带后缀的 bookinfo 文件也要查一次内容，排除错误响应
			if head, err := os.ReadFile(src); err == nil && strings.Contains(string(head), `"errcode"`) {
				if os.Remove(src) == nil {
					counts["dropped-err"]++
				}
				continue
			}
		}
		dstDir := filepath.Join("temp", sub)
		if err := os.MkdirAll(dstDir, 0755); err != nil {
			counts["mkdir-err"]++
			continue
		}
		dst := filepath.Join(dstDir, name)
		if _, err := os.Stat(dst); err == nil {
			counts["already-moved"]++
			continue
		}
		if err := os.Rename(src, dst); err != nil {
			counts["move-err"]++
			fmt.Fprintln(os.Stderr, "移动失败:", src, err)
			continue
		}
		counts[sub]++
	}
	// 二次扫描：清理已归档子目录里残留的错误响应（首次归档时后缀分支未查内容）
	for _, sub := range []string{"bookinfo", "booklist", "bookmark"} {
		dir := filepath.Join("temp", sub)
		fs, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range fs {
			if e.IsDir() {
				continue
			}
			p := filepath.Join(dir, e.Name())
			if head, err := os.ReadFile(p); err == nil && strings.Contains(string(head), `"errcode"`) {
				if os.Remove(p) == nil {
					counts["dropped-err"]++
				}
			}
		}
	}
	fmt.Println("归档完成:", counts)
}
