# HTML/XPath 核心来源与修补

- `xpath`：复制自 `github.com/antchfx/xpath v1.3.8` 的根目录非测试 Go 文件。
- `htmlquery`：复制自 `github.com/antchfx/htmlquery v1.3.4` 的根目录非测试 Go 文件，导入改为项目内的 XPath。
- 两个目录均保留各自完整的 MIT LICENSE。其他依赖由根目录 go.mod 管理。

原 v1.3.3 的 substring 对起点小于 1 会 panic，导致短文本参与后缀筛选时程序崩溃。v1.3.8 修复了该路径，但字符串长度、截取及 translate 仍按 UTF-8 字节处理部分索引。

本地 `xpath/func.go` 修补：

1. `substring` 按 Unicode 码点和 XPath 1.0 的位置/舍入规则取字符，处理负数、NaN 和无穷值。
2. `string-length` 返回码点数。
3. `translate` 按码点建立映射，重复源字符使用首次映射。

`TestPythonStaticReference` 使用原 Python/lxml 运行结果验证这些差异。更新上游时应重新核对这三处修补；不能只替换模块缓存或根项目 replace，因为这会使下游安装丢失修复。
