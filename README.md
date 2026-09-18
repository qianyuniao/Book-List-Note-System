# weread-hot-booklists

热门收藏书单工具集：Go 爬虫生成书单划线笔记归档 + 本地 Web/桌面浏览器（Go 单语言，零第三方依赖）。

- **技术栈**：Go 1.16+（纯标准库、零第三方依赖，前端页面 go:embed 内置）
- **上游数据项目**：`github.com/able8/weread-hot-booklists`（Star 785 / Fork 261）

## 项目结构

```
.
├── cmd/
│   ├── scraper/main.go      # 爬虫：抓取微信读书 API，生成 INDEX.md 与 books/ 笔记
│   └── server/              # 本地 Web 服务（默认端口 8327）兼桌面程序
│       ├── main.go          #   JSON API + 启动自动打开 Edge 应用模式窗口
│       └── index.html       #   前端单页（go:embed 内置，书单浏览/编辑/添加/删除）
├── start-desktop.bat        # 双击启动桌面窗口（有 exe 用 exe，否则 go run）
├── books/                   # 数据产物：100 个书单目录，约 3000 篇划线笔记 md
├── INDEX.md                 # 数据产物：书单总索引（由爬虫生成，勿手改）
├── temp/                    # 爬虫 API 响应缓存（.gitignore 排除，可再生），按类型分目录：
│   ├── booklists.json       #   书单榜总览
│   ├── bookinfo/            #   书籍元信息 <bookId>.json
│   ├── booklist/            #   单书单详情 <booklistId>.json
│   └── bookmark/            #   热门划线 <bookId>.json
├── cookie.txt.example       # 登录凭证模板（复制为 cookie.txt 后填写）
├── go.mod
└── .gitignore
```

## 快速开始

### 桌面窗口（推荐，双击即用）

双击 `start-desktop.bat`，自动弹出独立的应用窗口（Edge 应用模式，无地址栏，观感同原生软件）：

| 功能 | 说明 |
| :--- | :--- |
| 三栏浏览 | 书单（可搜索）→ 书籍 → 渲染后的划线笔记 |
| 右侧工具条 | ⌂ 阅读｜✎ 编辑（Ctrl+S 保存）｜✚ 添加书籍｜✕ 删除书籍 |
| 未保存保护 | 切换/刷新时弹窗确认，关闭页面前拦截 |
| 刷新 | 标题栏 ⟳ 或 F5 重扫 books/ 目录 |

窗口本质是本地网页，也可手动在任意浏览器访问 http://localhost:8327。

命令行启动（项目根目录）：

```powershell
go run ./cmd/server            # 首次稍等编译；可选参数：-addr :9000 -dir books -no-browser
```

### 爬虫（重新生成数据）

```powershell
go run ./cmd/scraper           # 必须在项目根目录运行
```

> 仓库不内置编译产物（`*.exe` 已被 .gitignore 排除）。需要 exe 时自行构建：
> `go build -o weread-server.exe ./cmd/server`、`go build -o weread-scraper.exe ./cmd/scraper`
> 构建好后 `start-desktop.bat` 会直接双击启动，无需 Go 环境。

- **缓存模式**（默认）：`temp/` 命中时约 3 秒离线完成。
  ⚠️ 重抓会重写 `books/` 下全部笔记，**手工补全的简介/观点/金句章节会被覆盖**，请谨慎执行。
- **实时抓取**：复制 `cookie.txt.example` 为 `cookie.txt`，填入浏览器抓取的登录 Cookie
  （单行 `Cookie: wr_vid=...; wr_srkey=...`，**末尾不能有多余换行**），再按需删除对应缓存：

| 刷新范围 | 删除的缓存 |
| :--- | :--- |
| 仅书单榜排名 | `temp/booklists.json` |
| 指定书单 | `temp/booklist/<booklistId>.json` 及相关 `temp/bookmark/`、`temp/bookinfo/` 文件 |
| 全量重抓 | 整个 `temp/`（约 4700 请求，注意限流） |

> 历史遗留的扁平命名缓存可用 `go run .\migratetemp` 一键归档到上述子目录（幂等，可重复执行）。

## API 一览（weread-server）

- `GET /api/booklists` — 书单列表 `[{name, count}]`
- `GET /api/books?list=书单名` — 书籍列表
- `GET /api/content?list=书单名&title=书名` — 笔记 Markdown
- `POST /api/save` `{list,title,content}` — 保存笔记修改
- `POST /api/add` `{list,title,author,intro}` — 新增书籍笔记（文件名自动过滤非法字符，同名拒绝）
- `POST /api/delete` `{list,title}` — 删除书籍笔记

所有写接口均带路径穿越防护（结果必须落在 books/ 内）。

## 数据说明

- 榜单来源：`https://i.weread.qq.com/market/booklists?count=200&type=0`
- 每本书生成 `books/<书单名>/<书名>.md`：作者、分类、简介 + 按章节排序的热门划线（`c:N` 为划线人数）
- 划线人数 > 300 的金句摘录进 `INDEX.md` 索引

## 常见问题

| 现象 | 原因与解决 |
| :--- | :--- |
| `无法将.\xxx.exe项识别为...` | 当前目录不对，先 `cd` 到项目根目录 |
| 双击 bat 没有弹窗口 | 未找到 Edge/Chrome，手动访问 http://localhost:8327 |
| 弹窗 0xc0000142 | 进程初始化偶发失败，点确定后重新启动即可 |
| 响应含 `errcode` 直接退出 | Cookie 失效或限流，重新登录获取、稍后再试 |
| 端口 8327 被占用 | `go run ./cmd/server -addr :其他端口` |

## 注意事项

- `cookie.txt`、`temp/`、`*.exe` 已被 `.gitignore` 排除，**切勿提交个人登录凭证**
- `INDEX.md` 与 `books/` 是爬虫产物，重新运行爬虫会覆盖
