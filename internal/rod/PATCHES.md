# Rod 核心修补版

来源：`github.com/go-rod/rod v0.116.2` 模块根目录的非测试 Go 源文件。MIT 许可证完整保留在本目录 LICENSE。CDP 协议、输入键、启动器等子包继续使用 go.mod 锁定的上游版本。

本项目在自己的 `internal/rod` 中编译这些文件，避免修改 Go 模块缓存，也避免依赖不会向下游传播的 replace/vendor 配置。

当前两处修补：

1. `page_eval.go`：iframe 切换进程时 ContentDocument 可能为空，返回可重试的上下文错误，避免空指针崩溃。
2. `page.go`：WaitRepaint 使用调用方 context，后台标签页暂停动画帧时仍遵守操作超时。

回归：`TestFrameRebindingAndFileDrop` 验证同源/跨站往返与文件拖入；`TestAsyncHTMLDemo/12_navigation` 验证新标签页打开后的原页操作。迁移依赖升级时需重新核对这两处差异。
