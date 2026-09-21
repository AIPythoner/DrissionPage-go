# 原始 HTML 测试台

页面和上传样本复制自 `E:\code\open-source\DrissionPage-async\tests\demo`，复制日期 2026-09-21。原目录保持不变。版权和使用条件遵循项目根目录 LICENSE。

- `site/index.html`：主页面，保持原始内容。
- `site/frame.html`、`nested.html`、`page2.html`：配套页面，保持原始内容。
- `files/upload1.txt`、`upload2.txt`：上传样本。

Go 测试入口为根目录 `async_demo_test.go`，使用本机临时 HTTP 服务和真实安装的 Chrome。API 服务由 Go 实现；测试不调用 Python，也不依赖源目录继续存在。

执行：`powershell -NoProfile -File scripts/test-demo.ps1`。默认无头运行，`-Headful` 可显示浏览器窗口。

Go 用例覆盖 18 个场景组：原页面的 17 个功能区，以及会话、模式切换和并发。分组通过不等于原异步 Python 版 488 条断言逐项等价通过。原版特有的 await 链式语法不直接复制到 Go。
