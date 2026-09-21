# 验证记录

日期：2026-09-21。

## 最新验证结果

- Windows / Go 1.26.4：`go test -count=1 -timeout 3m -json ./...`，**19 个顶层测试组及其子用例通过**，总测试耗时 48.795 秒。
- `TestAsyncHTMLDemo` 的 **18 个场景组全部通过**，页面源于原异步版 HTML 测试台；范围见 [覆盖说明](HTML_DEMO.md)。
- 设置了 `DRISSIONPAGE_BROWSER` 和 `DRISSIONPAGE_FFMPEG`，真实 Chrome 与 MP4 导出用例执行，测试用例跳过数为 0。
- 新增 frame 同源/跨站往返与文件拖入测试，修复驱动缺陷后独立连续运行三次通过，汇总运行再次通过。
- 重连、重定向逐跳 extra-info 配对、队列溢出、会话认证/参数/编码/代理/TLS/重定向限制测试通过。
- `go vet ./...` 通过。
- Linux amd64、macOS arm64 的 `CGO_ENABLED=0 go build ./...` 均通过。
- 四个 HTML 副本 SHA-256 分别与原异步项目中的同名文件相同。
- 完整机器可读结果：[test-results-2026-09-21.jsonl](test-results-2026-09-21.jsonl)。

Linux/macOS 仅验证交叉编译，没有在对应系统运行浏览器测试。

## 未通过或未完成

- Windows `go test -race ./...` 在测试程序启动时退出，状态 `0xc0000139`。未获得竞态检测结果。
- Python 原版全部 API、参数、错误语义的逐项等价对照尚未完成。
- 未实现项和行为差异见 [迁移清单](MIGRATION.md)。

## 源码与交付状态

- Python 同步版源码目录：`E:\code\open-source\DrissionPage`，原工作区保持干净。
- 异步版源码目录：`E:\code\open-source\DrissionPage-async`，原工作区保持干净。
- Go 项目目录：`E:\code\open-source\DrissionPage-go`，已建立独立 Git 仓库。
- 原版和 Go 项目的 LICENSE 文件 SHA-256 一致：`299744C4F3C91074CEA338F7386B4D66061423059DF938CAFD4026FD03CF422D`。
- 上述测试完成时尚未提交或推送。后续按用户要求将当前开发版本提交至 `https://github.com/AIPythoner/DrissionPage-go`；提交与推送记录以 Git 历史为准，尚未发布正式版本或部署。
