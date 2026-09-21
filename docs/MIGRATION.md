# 迁移范围与 Go 对应

源代码：DrissionPage **5.0.0b1 / b46345b**。Go 项目独立存放于 `E:\code\open-source\DrissionPage-go`，原同步/异步项目不写入迁移代码。

原类公开成员见 [API 目录](API_INVENTORY.md)。Go 使用结构体字段、显式方法、切片、context 和 error 承接 Python 的属性、链式 setter 与动态返回类型。

## 模块对应

| 原模块 | Go 对应 | 交付能力 |
| --- | --- | --- |
| driver / browser | `chromium.go`、`internal/rod` | 启动、接管、CDP、连接/进程生命周期；内部 Driver/TargetManager 由 Go 与 Rod 会话管理承接 |
| Chromium / Context | `Chromium`、`advanced.go` | 标签页、独立 context/代理、Cookie、排序、查找/关闭/激活、进程 ID、命令行、状态、缓存、断开/重连 |
| ChromiumOptions / INI | `options.go`、`profile.go`、`config.go`、`browser_compat.go` | flags 合并/清空、偏好删除、非 Default profile、系统 profile 副本、临时/下载目录、Edge、端口、仅接管、INI/JSON |
| SessionOptions / SessionPage | `session*.go` | HTTP、请求体/上传、Cookie、参数/认证、编码、代理、TLS/CA/证书、重定向、重试、hooks、最长前缀 adapter、流式响应 |
| ChromiumBase / Tab | `tab.go`、`advanced.go`、`metadata.go` | 导航、运行时超时/加载模式/重试、JS/CDP、截图、PDF/MHTML、存储、资源/blob、页面元信息 |
| 双模式页面 | `HybridPage`、`ToSession` | HTTP/浏览器切换、Cookie/URL/UA 同步 |
| ChromiumFrame | `frame*.go` | owner 元素代理、frame 导航/移除、同源/OOPIF、逐次重绑定；URL 在进程切换时重试；context/error 页面接口生成代理 |
| SessionElement / ChromiumElement / ShadowRoot | 元素文件、`metadata.go` | 定位、属性/文本/HTML、链接/注释/子文本、值/样式/属性 setter、路径、资源、快照、开放/关闭 shadow、伪元素 |
| locator / text | `locator.go`、`query.go`、`text.go`、`internal/xpath` | CSS/XPath/属性组合/文本/AX/原生搜索；中文 XPath 函数经过原版对照 |
| 关系 / 元素列表 | `Relative` / `Relatives`、`ElementFilter`、命名切片 | XPath 关系轴、正负索引、条件过滤、首项、批量文本/属性；Go 谓词承接链式组合 |
| JS 返回对象 | `RunJS`、`RunExpression`、`RunAsyncJS`、`EvalHandle` | 语句体/函数/脚本文件、元素参数、Promise、异步结果、DOM/数组/对象句柄 |
| actions / clicker | `Actions`、元素点击方法、`Direction` / `Offset` | 左/右/中/多击、键盘、悬停、拖动、文件拖入、点击上传/下载/新页/URL/标题变化 |
| waiter | `WaitState`、`WaitUntil`、`waiters.go` | 删除/显隐/覆盖/启用/可点击/矩形/停止移动、任意或全部定位器、加载/URL/标题、弹窗、下载、新标签 |
| states | `ElementStates`、`BrowserStates`、`WatchState` | 元素/浏览器状态、页面加载及弹窗事件观察 |
| rect / scroller | `Rect`、`PageGeometry`、`FrameGeometry`、`RootRect`、`ScreenRect`、`BoxModel` | 元素/页面/frame/根视口、屏幕位置、原始四边形、窗口、方向滚动及居中 |
| selector | `selector.go` | 文本/value/CSS/索引、多选追加、取消、全选/反选/清空、选项与选中项 |
| listener / packets | `listener.go`、`browser_listener.go`、`packet_extras.go` | 请求/响应/正文/失败/FrameID、重定向逐跳 extra-info、POST 正文、WS 握手/消息、SSE、动态过滤、暂停/恢复/静默 |
| console | `Console.Next` / `Wait` / `Messages` / `Clear` / `Stop` | 控制台队列、对象快照；Go 消费循环替代 steps 生成器 |
| downloader | `DownloadMission`、`DownloadManager` | 流下载、进度/取消、GUID、frame 关联、WaitBegin/WaitAll、重命名/覆盖/跳过 |
| cookies / permissions | `cookies.go`、`SetPermission` | Cookie 元信息/删除/清空/同步；通用权限名替代每个权限的便利属性 |
| window / setters | `SetWindow`、`HideWindow` / `ShowWindow`、`ShowTrail` | CDP 窗口状态、Windows 原生窗口、鼠标轨迹、阻止 URL 等 |
| screencast | `StartRecording`、`StartDisplayRecording`、`Video` / `TimelineVideo` | imgs/video 定时采集、frugal 重绘采集、js_video 屏幕共享、固定 FPS 或真实时间间隔导出 |
| errors / NoneElement / Settings | error、context、各对象 options | 显式错误和选项；错误展示语言由调用应用决定 |
| CLI / Python 对象互操作 | `cmd/drissionpage`、CDP 地址连接 | CLI 抓取；跨语言通过 CDP 接管浏览器 |

## 使用与兼容边界

1. 无元素返回 `ErrElementNotFound`。XPath 标量/文本/属性用 `XPathValues`，DOM 对象用 `EvalHandle`；不提供 Python 空对象的任意属性链回退。
2. `NewChromium(Address)` 仅连接；本机地址不存在时自动启动用 `ConnectOrLaunch`。重连后重新取得 tab/元素并安装监听。生命周期和同一 tab 的配置变更应与操作串行。
3. 系统 profile 复制到独立目录，`NewEnv` 使用新目录。保存配置必须指定目标路径，不写入依赖目录的默认 INI。Python hooks/adapter 改用 Go 函数/RoundTripper；流式请求用 `OpenStream`，普通 Get/Post 返回已读取的 Response。
4. 静态 CSS 受 Cascadia 支持范围限制，浏览器 CSS 由 Chromium 处理。`Neighbor` 按距离排序；原版八像素射线扫描用 `Direction`。`RootRect` 支持轴向缩放，旋转/倾斜 frame 返回错误；精确四边形用 `BoxModel`。原生屏幕坐标需要本机 Windows 有头窗口。
5. 输出队列默认 4096 条，超限丢弃旧项并计数。活动请求/extra-info 默认上限 4096，超限停止并返回 `ErrListenerCapacity`。本库 NewTab 在导航前安装浏览器监听；外部创建页的早期事件可能发生在订阅前，不能事后恢复。
6. 同 frame 的库内点击下载串行关联。外部并发产生多个下载时，通过 URL 条件或 GUID 选择；歧义报错，不猜测归属。
7. js_video 使用浏览器屏幕共享选择器及权限；页面导航会使 JS 录制句柄失效。FFmpeg/ffprobe 由调用方提供。观察器、流响应、监听和 JSHandle 均需按接口说明释放。

## 验收证据

- 原 HTML 测试台 18 个事件场景组：[HTML_DEMO.md](HTML_DEMO.md)。
- 原 Python 生成的 22 个定位、9 个文本、9 个 XPath 标量结果，共 40 项：`testdata/reference.json`。
- 配置目录、HTTP hooks/adapter/stream、录屏文件、Windows 窗口、frame 重绑定/几何、JS 句柄、动态监听等另有集成测试。
- 完整测试、竞态检测和平台结果：[VERIFICATION.md](VERIFICATION.md)。

以上是明确的功能及验收范围。没有把模块数换算成“100%”，也没有把 18 个场景组表述为原 Python 488 条断言全部通过。原版所有参数组合和错误语义的逐项等价验证尚未完成；API 目录中的 858 个公开成员也不是 858 项已独立验收的测试。
