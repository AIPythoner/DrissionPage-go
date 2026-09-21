# 验证记录

日期：2026-09-21。环境：Windows、Go 1.26.4、实际 Chrome、FFmpeg/ffprobe。

## 本机验证

- 常规测试：25 个顶层测试组，连同子测试共 95 条通过记录；失败 0，测试用例跳过 0。耗时 47.089 秒。
- 竞态检测：相同测试范围已通过；最终结果与耗时以 [race-results-2026-09-21.jsonl](race-results-2026-09-21.jsonl) 为准。
- 原 HTML 测试台：18 个场景组全部执行，覆盖范围见 [HTML_DEMO.md](HTML_DEMO.md)。
- Python 参考结果：22 个定位、9 个格式化文本、9 个 XPath 标量用例，40 项一致。原版运行脚本：`scripts/python-reference.py`，测试数据：`testdata/reference.json`。
- 新增：配置副本/flags/偏好、HTTP hooks/最长前缀 adapter/stream、Windows 窗口隐藏恢复/屏幕位置、定时录屏/真实时间轴/屏幕共享 WebM、FFprobe 解析、frame 进程切换/几何/存储、JS 句柄、多选追加/取消、动态监听/容量、弹窗观察与等待。
- iframe 进程切换相关测试在修复短暂失效错误后连续运行 3 次通过。
- `go vet ./...` 通过。Linux amd64、macOS arm64 的 `CGO_ENABLED=0 go build ./...` 通过。
- 普通测试日志：[test-results-2026-09-21.jsonl](test-results-2026-09-21.jsonl)。

“95 条”包含父测试与子测试，不能解读为 95 个相互独立场景。没有测试文件的内部包会显示 package skip，不计入用例跳过数。

## 竞态检测工具链

早期使用旧 MinGW GCC 8.1 时，测试程序启动即遇到 `0xc0000139`。本轮使用 LLVM-MinGW **20260908 / UCRT x86_64** 后，实际完成 `go test -race` 并通过。

工具链下载自 mstorsjo/llvm-mingw 官方发布仓库，存于被 Git 忽略的 `.tools`。这是本机验证工具，不是运行库依赖；工具链文件不提交。

## 复现

```powershell
$env:DRISSIONPAGE_BROWSER = 'C:\Program Files\Google\Chrome\Application\chrome.exe'
$env:DRISSIONPAGE_FFMPEG = 'E:\Apps\ffmpeg\ffmpeg.exe'
go test -count=1 -timeout 4m -v ./...
go vet ./...

# 竞态检测另需支持的 C 编译器
$env:CC = (Resolve-Path '.tools/llvm-mingw-20260908-ucrt-x86_64/bin/clang.exe').Path
$env:PATH = (Resolve-Path '.tools/llvm-mingw-20260908-ucrt-x86_64/bin').Path + ';' + $env:PATH
$env:CGO_ENABLED = '1'
go test -race -count=1 -timeout 5m -v ./...
```

原版对照数据可重新生成：

```powershell
python -B scripts/python-reference.py E:\code\open-source\DrissionPage
go test -run TestPythonStaticReference .
```

## 平台与兼容性边界

本机实际运行的是 Windows 浏览器测试。Linux 已通过 GitHub Actions 的 Go 1.23.x / 1.26.x 两组真实浏览器和竞态测试，均完成录屏与跨目标编译，源码提交为 `9530453`：[工作流 35583111574](https://github.com/AIPythoner/DrissionPage-go/actions/runs/35583111574)，[结果摘要](ci-results-9530453.json)。macOS 原生浏览器/屏幕录制没有实机验收。

第一轮 Linux 验证发现屏幕共享未选到来源；修复为允许当前 tab 并按标题选择测试标签页后，两组均通过。参照 [Chrome 官方屏幕共享说明](https://developer.chrome.com/docs/web-platform/screen-sharing-controls)。此自动选择参数仅用于测试；正常接口仍由用户选择并授权。

原 Python 的 488 条 demo 断言没有逐条移植，全部 API 参数/错误语义也没有穷尽对照。已验收范围、Go 替代接口与已知差异见 [MIGRATION.md](MIGRATION.md)；不能从模块覆盖或测试通过数推导全量等价百分比。

## 源码与交付

- Go 项目：`E:\code\open-source\DrissionPage-go`。
- 原同步/异步项目工作区均保持干净。
- 原 LICENSE 原样保留，SHA-256：`299744C4F3C91074CEA338F7386B4D66061423059DF938CAFD4026FD03CF422D`。
- Rod、HTMLQuery、XPath 的许可证和本地修补记录均随源码提交。
- GitHub 目标：[AIPythoner/DrissionPage-go](https://github.com/AIPythoner/DrissionPage-go)。实际提交和推送状态以 Git 历史为准；没有创建正式版本标签。
