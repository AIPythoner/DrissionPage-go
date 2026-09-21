# 迁移清单

来源：`E:\code\open-source\DrissionPage`，Python 5.0.0b1，提交 `b46345b`。
目标：`E:\code\open-source\DrissionPage-go`。原仓库不写入 Go 代码。

状态说明：下表记录功能实现范围，并不表示 Python 全部公开方法已逐项等价验证。

| Python 模块 | Go 文件 | 已实现 | 仍需补齐 / 差异 |
| --- | --- | --- | --- |
| `_base/driver`、`_functions/browser` | `chromium.go` | CDP 连接、启动、生命周期、原始命令 | 内核采用 Rod；原内部 Driver/TargetManager 不逐类复刻 |
| `_browsers/chromium*` | `chromium.go`, `advanced.go`, `browser_listener.go` | 标签页、独立 context/代理、查找、关闭、Cookie、聚合监听 | 已支持 Disconnect/Reconnect，重连后需重新获取 tab/监听；LatestTab 对本库创建的标签页记录顺序，接管前的历史顺序不可恢复 |
| `_pages/session_page` | `session.go`, `session_helpers.go`, `cookies.go` | HTTP、Cookie/元信息、编码、重试、JSON、表单、文件、本地 HTML | 已补 headers/UA/timeout/retry/params/auth/encoding/proxy/TLS/redirect setter；hooks/adapter 等尚未逐项对应；WithOptions 创建共享 Cookie 的新配置会话，自定义 HTTP Client 配置 TLS/代理/重定向 |
| `_pages/chromium_base/tab` | `tab.go`, `advanced.go`, `hybrid.go` | 导航/重试、JS/CDP、截图、PDF/MHTML、存储、模式切换 | Go 类型使用 HybridPage 承接双模式；参数与错误行为尚需全量对照 |
| `_pages/chromium_frame` | `chromium_element.go`, `frame.go` | 同源/跨站 OOPIF、frame 内导航、元素代理、关闭移除 frame | 常用查找/脚本/滚动操作已按 owner 重绑定并测试同源/跨站往返；owner 被替换后需重新查找，其他继承方法尚需完整核对 |
| `_elements/session_element` | `session_element.go`, `text.go`, `relations.go` | HTML、属性、格式化文本、定位、树、关系、路径 | XPath 文本与属性使用 XPathValues；格式化文本仍需更多对照测试 |
| `_elements/chromium_element` | `chromium_element.go`, `advanced.go`, `query.go`, `geometry.go` | 点击、输入、属性、上传、拖动、frame、开放/关闭 shadow、快照、空间定位、缓存资源读取 | 空间定位与原算法的精确行为、部分 shadow-root 边界场景仍需对照 |
| `_elements/none_element`、`errors` | `errors.go` | 可判定错误及 HTTPError | Go 使用 error，不支持 Python 的空对象链式回退 |
| `_functions/locator` | `locator.go`, `query.go` | CSS/XPath/属性组合/文本/AX/原生搜索 | 复杂定位器需更多 Python 对照测试；静态 CSS 受 Cascadia 支持范围限制 |
| `_configs/*` | `config.go`, `options.go` | INI/JSON、常用参数、偏好、代理、扩展 | 系统配置目录复制、flags 文件、全量 INI 字段兼容待补齐 |
| `_units/listener` | `listener.go`, `browser_listener.go`, `packet_extras.go`, `events.go` | 请求/响应/正文/失败、extra-info、WS、SSE、暂停/恢复、浏览器聚合 | 已补重定向逐跳 extra-info 配对和输出队列上限；活动请求元数据不受输出队列上限控制；外部客户端创建标签页可能漏掉订阅前的早期事件 |
| `_units/downloader` | `download.go`, `download_policy.go` | HTTP 流下载、进度/取消、浏览器任务、命名/覆盖/跳过、点击下载 | 已补 frame 关联与辅助点击串行化，不消费公共事件；同 frame 外部并发下载仍需 GUID 核对；HTTP 直接下载保留不覆盖策略 |
| `_units/actions/clicker` | `units.go`, `advanced.go` | 动作链、左右点击、键盘、拖动、点击新标签 | 已补拖入文件和文件选择器上传；部分点击后等待组合待补齐 |
| `_units/waiter/states` | `units.go` | 状态、条件等待、删除/URL/标题/加载等待、被释放对象状态 | 仍有原 waiter 便利组合未逐个包装，可组合 WaitState/WaitUntil |
| `_units/rect/scroller` | `chromium_element.go`, `units.go`, `geometry.go` | 元素视口/页面矩形、窗口/页面几何、相对/绝对滚动 | 操作系统原生屏幕坐标及完整 frame 几何待补齐 |
| `_units/selector` | `chromium_element.go`, `selector.go` | CSS/文本/value/索引、全选/反选/取消选择、选中项 | 多选及未匹配值等边界行为还需原版对照 |
| `_units/setter/perm_setter` | `units.go`, `options.go` | 元素/存储/窗口/UA/Header、通用权限 | OS 原生窗口隐藏/显示、其他便捷 setter 待补齐 |
| `_units/console` | `listener.go` | 控制台事件队列 | 已补有限深度对象快照，循环引用/访问器/截断有标记；不是无限深度展开 |
| `_units/screencast` | `screencast.go` | JPEG 帧录制、FFmpeg 视频导出 | 固定 FPS 导出；原多种录制模式和真实时间轴待补齐 |
| `_functions/cli/common` | `cmd/drissionpage`, `advanced.go`, `relations.go` | CLI 抓取、绝对链接、通用等待、树 | Python 对象互操作改为 CDP 地址连接；其余平台工具待逐项核对 |

## 验证范围

已有测试包括静态定位/文本/关系/路径、索引、引号、HTTP 会话/Cookie 元信息与删除/重试/状态码、上传下载、文件冲突策略、浏览器输入点击、同源 iframe/跨进程 iframe、开放/关闭 shadow root、截图、超时、响应正文/extra-info、浏览器级首次导航捕获、控制台、存储、选择框、下载、context 隔离、WebSocket/SSE、Cookie 同步、双模式、AX、PDF/MHTML、录屏帧/FFmpeg 导出及 INI。

`go test -race` 在当前 Windows 机器启动时遇到 `0xc0000139`，尚未获得竞态检测结果。跨操作系统和 Python 原测试集的逐项对照尚未完成。

后续按表内待补齐项继续，不能把当前模块覆盖视为全量迁移完成。

## 进度估算与后续工作（2026-09-21）

此前按实现范围及已知缺口做工程粗估，功能实现约 **60%–75%**，剩余约 **25%–40%**。该历史估算未随本轮增量重算，不作为当前完成比例。这不是公开 API 逐项统计或验收通过率，尚不能给出精确百分比。剩余工作预计为数周量级，取决于兼容性对照发现的问题及跨平台环境可用性，不是交付期限承诺。

后续分五阶段推进：

1. 会话与配置：运行时设置、编码、认证、代理、重定向、全量 INI 与配置文件行为。
2. 浏览器生命周期：重连、frame 自动重绑定、窗口及 frame 几何。
3. 事件与下载：重定向 extra-info 配对、队列上限、并发下载关联、控制台对象展开。
4. 交互与录制：拖入文件、上传对话框、等待组合、录屏模式与时间轴。
5. 兼容性验收：公开 API 对照清单、定位器和错误等边界行为对照、跨平台运行及竞态检测。

100% 完成标准：公开功能逐项有 Go 实现或明确替代方式，缺口关闭且差异有说明，关键场景通过原版对照及目标平台验证。当前尚未达到。

本轮新增 `session_setter.go`：`SetHeaders`、`SetHeader`、`SetUserAgent`、`SetTimeout`、`SetRetry`。请求保存配置快照，设置影响后续普通 HTTP 请求。认证、编码等 setter 尚待补齐。新增本机 HTTP 测试覆盖请求头复制与覆盖、503 重试、超时、非法参数及关闭状态。

本轮已接入原异步版 HTML 测试台，18 个 Go 场景组通过，详见 [覆盖说明](HTML_DEMO.md)。新增重连、重定向、队列边界、frame 进程切换、文件拖入及会话传输配置测试。此结果仍不是全量 API 验收。

汇总验证：19 个顶层测试组及子用例通过，跳过 0。驱动核心来自 Rod v0.116.2，两个修补记录于 `internal/rod/PATCHES.md`。INI 另补扩展列表、profile 名称、加载/脚本超时和重试字段。
