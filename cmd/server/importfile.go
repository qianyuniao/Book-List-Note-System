package main

// 导入外部文件并转换为笔记 markdown：
// 支持 md / txt / html / docx / epub / pdf / json / csv / tsv，全部基于标准库零依赖实现。
// PDF 为尽力而为的文本层提取（FlateDecode + 文字操作符），扫描版/CID 字体无法转换。

import (
	"archive/zip"
	"bytes"
	"compress/zlib"
	"fmt"
	"html"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	reScript = regexp.MustCompile(`(?is)<script[^>]*>.*?</script>|<style[^>]*>.*?</style>`)
	reBlockP = regexp.MustCompile(`(?i)<(?:br|/p|/div|/h[1-6]|/li|/tr|/blockquote|/section)[^>]*>`)
	reTag    = regexp.MustCompile(`(?s)<[^>]+>`)
	reWT     = regexp.MustCompile(`(?s)<w:t[^>]*>(.*?)</w:t>`)
	reBTET   = regexp.MustCompile(`(?s)BT(.*?)ET`)
	rePDFLit = regexp.MustCompile(`\(((?:[^()\\]|\\.)*)\)`)
)

// handleImport 接收 multipart 上传（字段：list 目标书单、file 文件），转换后写入 books/<list>/
func handleImport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "仅支持 POST")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 64<<20)
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		writeErr(w, http.StatusBadRequest, "上传解析失败（文件过大或表单格式错误）")
		return
	}
	list := r.FormValue("list")
	if list == "" {
		writeErr(w, http.StatusBadRequest, "缺少目标书单")
		return
	}
	dir, ok := safeJoin(list)
	if !ok {
		writeErr(w, http.StatusBadRequest, "非法路径")
		return
	}
	if _, err := os.Stat(dir); err != nil {
		writeErr(w, http.StatusNotFound, "书单不存在")
		return
	}
	f, hdr, err := r.FormFile("file")
	if err != nil {
		writeErr(w, http.StatusBadRequest, "未收到文件")
		return
	}
	defer f.Close()
	data, err := io.ReadAll(f)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "读取文件失败")
		return
	}

	ext := strings.ToLower(filepath.Ext(hdr.Filename))
	base := strings.TrimSuffix(filepath.Base(hdr.Filename), ext)
	title := strings.TrimSpace(illegalChars.ReplaceAllString(base, "-"))
	if title == "" {
		title = "导入笔记"
	}

	var body string
	switch ext {
	case ".md", ".markdown", ".mdown", ".mkd":
		body = string(data)
	case ".txt", ".text", ".log":
		body = normalizeLines(string(data))
	case ".html", ".htm":
		body = htmlToMD(string(data))
	case ".docx":
		body, err = docxToMD(data)
	case ".epub":
		body, err = epubToMD(data)
	case ".pdf":
		body = normalizeLines(pdfToText(data))
		if len([]rune(body)) < 40 {
			writeErr(w, http.StatusBadRequest, "PDF 提取内容过少（可能是扫描版或内嵌 CID 字体的文件），暂无法转换")
			return
		}
	case ".json":
		body = "```json\n" + string(data) + "\n```"
	case ".csv":
		body = csvToMD(string(data), false)
	case ".tsv":
		body = csvToMD(string(data), true)
	default:
		writeErr(w, http.StatusBadRequest, "暂不支持 "+ext+" 格式（支持 md/txt/html/docx/epub/pdf/json/csv/tsv）")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "解析失败: "+err.Error())
		return
	}
	if strings.TrimSpace(body) == "" {
		writeErr(w, http.StatusBadRequest, "文件中没有可提取的文本内容")
		return
	}

	// 同名笔记自动追加序号：xx (2).md
	name := title + ".md"
	for i := 2; fileExists(filepath.Join(dir, name)); i++ {
		name = fmt.Sprintf("%s (%d).md", title, i)
	}
	md := fmt.Sprintf("## %s\n\n导入自 %s  -  \n\n> 由「导入文件」功能从 %s 转换生成，排版可能有少量损失，可进入编辑模式调整。\n\n%s\n",
		title, hdr.Filename, hdr.Filename, strings.TrimSpace(body))
	if err := os.WriteFile(filepath.Join(dir, name), []byte(md), 0644); err != nil {
		writeErr(w, http.StatusInternalServerError, "写入失败: "+err.Error())
		return
	}
	writeJSON(w, map[string]string{"ok": "true", "title": strings.TrimSuffix(name, ".md")})
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// normalizeLines 把按行硬换行的纯文本重排为以空行分隔的段落
func normalizeLines(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	var out []string
	var para []string
	flush := func() {
		if len(para) > 0 {
			out = append(out, strings.Join(para, " "))
			para = nil
		}
	}
	for _, ln := range strings.Split(s, "\n") {
		t := strings.TrimSpace(ln)
		if t == "" {
			flush()
			continue
		}
		para = append(para, t)
	}
	flush()
	return strings.Join(out, "\n\n")
}

func htmlToMD(s string) string {
	s = reScript.ReplaceAllString(s, "")
	s = reBlockP.ReplaceAllString(s, "\n")
	s = reTag.ReplaceAllString(s, "")
	s = html.UnescapeString(s)
	// 块级标签已换算行：每个非空行独立成段，不再合并
	var out []string
	for _, ln := range strings.Split(s, "\n") {
		if t := strings.TrimSpace(ln); t != "" {
			out = append(out, t)
		}
	}
	return strings.Join(out, "\n\n")
}

func docxToMD(data []byte) (string, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", err
	}
	var doc *zip.File
	for _, zf := range zr.File {
		if zf.Name == "word/document.xml" {
			doc = zf
			break
		}
	}
	if doc == nil {
		return "", fmt.Errorf("缺少 word/document.xml（不是合法的 docx）")
	}
	rc, err := doc.Open()
	if err != nil {
		return "", err
	}
	raw, err := io.ReadAll(rc)
	rc.Close()
	if err != nil {
		return "", err
	}
	var paras []string
	for _, chunk := range strings.Split(string(raw), "</w:p>") {
		var sb strings.Builder
		for _, m := range reWT.FindAllStringSubmatch(chunk, -1) {
			sb.WriteString(html.UnescapeString(reTag.ReplaceAllString(m[1], "")))
		}
		if t := strings.TrimSpace(sb.String()); t != "" {
			paras = append(paras, t)
		}
	}
	return strings.Join(paras, "\n\n"), nil
}

func epubToMD(data []byte) (string, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", err
	}
	var parts []string
	for _, zf := range zr.File {
		n := strings.ToLower(zf.Name)
		if !strings.HasSuffix(n, ".xhtml") && !strings.HasSuffix(n, ".html") && !strings.HasSuffix(n, ".htm") {
			continue
		}
		rc, err := zf.Open()
		if err != nil {
			continue
		}
		raw, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			continue
		}
		if md := htmlToMD(string(raw)); len([]rune(md)) > 30 {
			parts = append(parts, md)
		}
	}
	if len(parts) == 0 {
		return "", fmt.Errorf("epub 中未找到正文内容")
	}
	return strings.Join(parts, "\n\n"), nil
}

// pdfToText 尽力提取 PDF 文本层：解压缩流 → 抓取 BT..ET 间的字面字符串
func pdfToText(data []byte) string {
	var out []string
	pos := 0
	for {
		i := bytes.Index(data[pos:], []byte("stream"))
		if i < 0 {
			break
		}
		start := pos + i + len("stream")
		if start < len(data) && data[start] == '\r' {
			start++
		}
		if start < len(data) && data[start] == '\n' {
			start++
		}
		j := bytes.Index(data[start:], []byte("endstream"))
		if j < 0 {
			break
		}
		plain, err := zlibInflate(data[start : start+j])
		pos = start + j + len("endstream")
		if err != nil {
			continue
		}
		for _, blk := range reBTET.FindAllSubmatch(plain, -1) {
			var sb strings.Builder
			for _, m := range rePDFLit.FindAllSubmatch(blk[1], -1) {
				sb.WriteString(unescapePDF(m[1]))
			}
			if t := strings.TrimSpace(sb.String()); t != "" {
				out = append(out, t)
			}
		}
	}
	return strings.Join(out, "\n\n")
}

func zlibInflate(b []byte) ([]byte, error) {
	zr, err := zlib.NewReader(bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	defer zr.Close()
	var buf bytes.Buffer
	_, err = io.Copy(&buf, io.LimitReader(zr, 32<<20))
	return buf.Bytes(), err
}

func unescapePDF(b []byte) string {
	var sb strings.Builder
	for i := 0; i < len(b); i++ {
		c := b[i]
		if c != '\\' || i+1 >= len(b) {
			sb.WriteByte(c)
			continue
		}
		i++
		switch b[i] {
		case 'n':
			sb.WriteByte('\n')
		case 'r':
			sb.WriteByte('\r')
		case 't':
			sb.WriteByte('\t')
		case 'b':
			sb.WriteByte('\b')
		case 'f':
			sb.WriteByte('\f')
		case '\\', '(', ')':
			sb.WriteByte(b[i])
		default:
			if b[i] >= '0' && b[i] <= '7' {
				oct := []byte{b[i]}
				for k := 0; k < 2 && i+1 < len(b) && b[i+1] >= '0' && b[i+1] <= '7'; k++ {
					i++
					oct = append(oct, b[i])
				}
				v := 0
				for _, o := range oct {
					v = v*8 + int(o-'0')
				}
				sb.WriteByte(byte(v))
			} else {
				sb.WriteByte(b[i])
			}
		}
	}
	return sb.String()
}

func csvToMD(s string, tab bool) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	sep := ","
	if tab {
		sep = "\t"
	}
	var md strings.Builder
	rows := strings.Split(strings.TrimSpace(s), "\n")
	for i, row := range rows {
		cells := strings.Split(row, sep)
		md.WriteString("| " + strings.Join(cells, " | ") + " |\n")
		if i == 0 {
			md.WriteString("|" + strings.Repeat(" --- |", len(cells)) + "\n")
		}
	}
	return md.String()
}
