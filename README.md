# DrissionPage Go

基于 DrissionPage Python **5.0.0b1**（源码提交 `b46345b`）迁移的独立 Go 项目。

本项目提供浏览器控制、HTTP 会话、元素定位、事件监听、下载和录屏等 Go 接口。功能对应、语言差异和验收边界见 [迁移清单](docs/MIGRATION.md) 与 [原版 API 目录](docs/API_INVENTORY.md)。
原项目版权及使用条件保留在 [LICENSE](LICENSE)。开发仓库为 [AIPythoner/DrissionPage-go](https://github.com/AIPythoner/DrissionPage-go)，当前以源码交付，尚未发布带版本号的正式发行包。

## 环境

- Go 1.23 或更高版本。
- 浏览器功能需要本机 Chrome、Edge 或其他 Chromium 浏览器，也可以连接已有的 CDP 地址。
- 运行库不依赖 Python、Selenium 或 WebDriver。
- 浏览器驱动基于 [Rod](https://github.com/go-rod/rod)，核心修补及来源见 [驱动说明](internal/rod/PATCHES.md)，HTML XPath 使用 [htmlquery](https://github.com/antchfx/htmlquery)，CSS 使用 Cascadia。

## HTTP 会话

支持运行时调用 `SetHeaders`（替换默认请求头）、`SetHeader`、`SetUserAgent`、`SetTimeout` 和 `SetRetry`，调用方应检查返回的错误。设置影响后续普通 HTTP 请求，单次请求头优先于默认值；`SetTimeout(0)` 关闭客户端超时，请求 context 仍可设置期限。

```go
package main

import (
    "context"
    "fmt"
    "log"
    dp "github.com/AIPythoner/DrissionPage-go"
)

func main() {
    page, err := dp.NewSessionPage()
    if err != nil { log.Fatal(err) }
    defer page.Close()
    if _, err = page.Get(context.Background(), "https://example.com"); err != nil {
        log.Fatal(err)
    }
    element, err := page.Ele("tag:h1")
    if err != nil { log.Fatal(err) }
    fmt.Println(element.Text())
}
```

## 浏览器

```go
ctx := context.Background() // 实际应用可使用 WithTimeout / WithCancel
options := dp.NewChromiumOptions().SetHeadless(true)
// Windows 可明确指定：options.SetBrowserPath(`C:\Program Files\Google\Chrome\Application\chrome.exe`)
browser, err := dp.NewChromium(ctx, options)
if err != nil { return err }
defer browser.Close()
tab, err := browser.NewTab(ctx, "https://example.com")
if err != nil { return err }
element, err := tab.Ele(ctx, "tag:h1")
if err != nil { return err }
text, err := element.Text(ctx)
if err != nil { return err }
fmt.Println(text)
```

`NewChromium` 的 context 控制浏览器连接的整个生命周期。`Address` 非空时连接已有浏览器；此时 `Close()` 断开连接，`Quit(ctx)` 才关闭浏览器。新启动浏览器使用独立临时用户目录，关闭后清理；显式指定的用户目录会保留。

查找和操作方法接收独立 context，支持取消。索引从 1 开始，负数倒数，0 返回 `ErrInvalidIndex`。未找到元素返回可用 `errors.Is(err, dp.ErrElementNotFound)` 判断的错误。

## 定位

| 示例 | 含义 |
| --- | --- |
| `css:div.item` / `dp.CSS("div.item")` | CSS |
| `xpath://div[1]` / `dp.XPath("//div[1]")` | XPath |
| `@id=submit` | 属性精确匹配 |
| `tag:button@@type=submit@!disabled` | 标签、与条件、否定条件 |
| `@|id=first@|id=second` | 或条件 |
| `text=提交` / `text:提交` | 精确文本 / 包含文本 |
| `text^开始` / `text$结束` | 文本前缀 / 后缀 |
| `ax:role=button@name=提交` | 浏览器无障碍定位 |
| `#submit` / `.item` / `关键词` | 浏览器原生搜索；静态页面支持简写 CSS 和文本包含 |

`t:`、`tx:`、`c:`、`x:` 支持对应缩写。XPath 标量、属性和文本结果使用 `XPathValues`；元素句柄使用 `Ele` / `Eles`。

## 模式切换与辅助功能

- `tab.Hybrid(ctx)` 创建 `HybridPage`；`ChangeMode(ctx, "s", true, true)` 切到会话模式并同步 URL/Cookie，`"d"` 切回浏览器。
- `tab.ToSession(ctx, true)` 创建独立会话并复制 Cookie、User-Agent。
- `tab.Listen(ctx, filter)` 监听网络请求；`Next` 取完整响应，`NextStream` 取 WebSocket/SSE 消息；使用结束后 `Stop()`。
- `tab.Console(ctx)` 捕获控制台消息；使用结束后 `Stop()`。
- `page.Download(ctx, url, path)` 流式下载，`mission.Wait(ctx)` 等待，`Cancel()` 取消。已有目标文件会报错，失败不发布半成品。
- `browser.Downloads(ctx, directory)` 监听浏览器下载，文件先用 GUID 命名；`Finalize` 按建议文件名或自定义名称处理重命名、覆盖、跳过策略。
- `tab.StartScreencast(ctx, newDirectory)` 保存重绘帧；`Stop(ctx)` 结束。`Video(ctx, ffmpegPath, output, fps)` 使用本机 FFmpeg 导出视频，按固定帧率播放。
- `LoadINI(path)` 读取原项目 INI 的字符串、列表、字典、布尔等常用配置；不执行配置中的 Python 表达式。Go 原生配置也可以直接 JSON 保存。

## 命令行

```powershell
go run ./cmd/drissionpage -version
go run ./cmd/drissionpage -url https://example.com -locator "tag:h1"
go run ./cmd/drissionpage -browser -url https://example.com -locator "tag:h1"
go build -o drissionpage.exe ./cmd/drissionpage
```

其他本地 Go 项目可使用 `replace github.com/AIPythoner/DrissionPage-go => E:/code/open-source/DrissionPage-go` 并添加 `require github.com/AIPythoner/DrissionPage-go v0.0.0`。

## 验证

### 原异步版 HTML 测试台

已接入 `DrissionPage-async/tests/demo/site/index.html` 和配套 iframe 页面。Go 场景覆盖定位、表单、各类点击、拖拽、键盘、等待、滚动、frame、shadow、弹窗、网络、导航、下载、console、存储及并发。

```powershell
./scripts/test-demo.ps1
# 显示浏览器窗口：
./scripts/test-demo.ps1 -Headful
```

当前这套页面的 18 个 Go 场景组已实际通过，默认使用真实 Chrome 无头运行。它不等于原 Python 版 488 条断言逐项通过，详细范围见 [HTML 测试说明](docs/HTML_DEMO.md)。

### HTML 测试页面与验证结果

2026-09-21 最新汇总：Windows / Go 1.26.4，**25 个顶层测试组及其子用例全部通过**，其中原 HTML 测试台的 18 个场景组全部通过。已指定真实 Chrome 和 FFmpeg，浏览器用例未跳过；完整结果保存在 [测试日志](docs/test-results-2026-09-21.jsonl)。

早期测试页面嵌入 Go 测试代码。现已另外接入原异步版完整 HTML 测试台，页面副本位于 `testdata/async-demo/site/index.html`，由 Go 本机临时 HTTP 服务提供。测试覆盖元素定位与文本、表单操作、Shadow DOM、iframe、HTTP 会话、Cookie、网络监听、下载及模式切换等场景。

`go vet ./...` 和 Linux amd64、macOS arm64 交叉编译均通过；后两者尚未在对应系统运行浏览器测试。Windows 竞态检测已通过，结果见 [竞态日志](docs/race-results-2026-09-21.jsonl)。检测使用 LLVM-MinGW 20260908 UCRT 工具链；运行库本身不要求 C 编译器。

上述结果代表已列明的 Go 用例通过。另有 40 项定位、文本与 XPath 标量结果与原 Python 实际运行结果对照。原版全部参数组合与错误语义没有穷尽验证；详细证据见 [验证记录](docs/VERIFICATION.md)。

### 运行测试

```powershell
go test ./...
$env:DRISSIONPAGE_BROWSER = 'C:\Program Files\Google\Chrome\Application\chrome.exe'
$env:DRISSIONPAGE_FFMPEG = 'E:\Apps\ffmpeg\ffmpeg.exe' # 可选，验证 MP4 导出
go test -v ./...
go vet ./...
```

未设置 `DRISSIONPAGE_BROWSER` 时，真实浏览器用例会明确标记跳过。测试站点均在本机临时启动。构造方法、操作方法返回错误，不使用 Rod 的 `Must*` 方法。

监听输出队列默认每个保留 4096 条事件，可用 `ListenFilter.BufferSize` 设置；溢出丢弃最旧事件，可通过 `PacketStats` / `StreamStats` 查看计数。活动请求/extra-info 默认上限 4096，可用 `MaxActiveRequests` 调整，超限停止并返回 `ErrListenerCapacity`；正文大小仍需按采集场景管理。配置、HTTP 会话及 UI 动作的并发修改应由调用方串行安排；不同标签页可以并行操作。

## 本轮迁移补充

- 浏览器 `Disconnect` / `Reconnect`：重建连接后需用 `GetTab` 重新获取标签页，并重新启动监听。
- frame 常用查找、脚本和滚动方法在操作前重新获取会话，支持保留 iframe 元素时的进程切换。
- 重定向各跳转的网络附加信息分开关联；监听队列可限长并查询丢弃计数。
- 点击下载按 frame 关联，避免消费其他下载的事件；同 frame 的辅助点击串行执行。外部代码在同 frame 同时触发下载时仍需按 GUID 核对。
- `ClickToUpload` 处理文件选择器，`DropFiles` 投放本地文件。
- 会话新增 `SetParams`、`SetAuth`、`SetEncoding`、`SetProxies`、`SetTrustEnv`、`SetTLSConfig`、`SetVerifyTLS`、`SetClientCertificate`、`SetMaxRedirects`。

原版属性/链式 setter 在 Go 中使用方法、结构体字段、切片和 `error`，具体差异以 [迁移清单](docs/MIGRATION.md) 为准。

## 补全交付的接口

- **配置与启动**：`SetFlag`、`ClearFlagsInFile`、`RemovePrefFromFile`、`CopySystemProfile`、`UseSystemProfile`、`SetUser`、`SetTempPath`、`SetDownloadPath`；`ConnectOrLaunch` 提供本机端口连接或启动行为。
- **会话扩展**：`SetResponseHooks`、`Mount`、`OpenStream`。hooks 可修改响应；adapter 按最长 URL 前缀选择；流响应由调用方关闭 Body。
- **frame 与 JS**：frame 页面接口重新绑定会话；`RootRect`/`Geometry`/`ScreenRect`/`BoxModel` 获取不同坐标；`EvalHandle` 保留 DOM/对象句柄，使用后 `Release`。`RunJS` 接受函数、语句体或脚本文件。
- **等待与状态**：`WaitElements`、`WaitStopMoving`、`WaitDisabledOrDeleted`、`DuringLoadStart`、`DuringAlert`、`WatchState`、`WaitBegin`、`WaitAll`。事件等待在 action 前订阅，观察器结束后 `Stop`。
- **录制**：`StartRecording(ctx, dir, "video")` 定时采集；`"frugal_video"` 按重绘采集。停止后 `TimelineVideo` 保留真实帧间隔；`StartDisplayRecording` 对应屏幕共享录制，`Stop(ctx, "capture.webm")` 保存文件。
- **交互**：`MiddleClick`、`MultiClick`、`Direction`、`Offset`、多选框追加/取消选择、`ShowTrail`、Windows 原生 `HideWindow`/`ShowWindow`。

录屏集成测试需要同目录的 FFmpeg 和 ffprobe，并会启动专用有头测试浏览器。屏幕共享的自动选择参数只用于测试，库正常调用仍使用浏览器权限流程。

源码中的 Rod 与 XPath 修补随本模块发布，下游 `go get` 不需要额外 replace 或修改模块缓存。来源见 [Rod 修补说明](internal/rod/PATCHES.md) 和 [XPath 修补说明](internal/XPATH_PATCHES.md)。

GitHub Actions 配置执行 Linux 浏览器/竞态测试及其他目标的编译；CI 是否通过以对应提交的工作流结果为准。
