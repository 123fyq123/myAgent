# common 说明

## 模块定位
`common` 是后端公共工具库，提供业务错误码、Markdown/EPUB/文件处理和 token 计数等跨模块复用能力。

## 主要职责
- 维护统一业务错误码。
- 提供 Markdown 文本处理与分片辅助。
- 提供 token 计数能力。
- 提供文件与 EPUB 相关工具函数。

## 目录结构

### main.go / 入口文件
- 本模块没有 `main.go` 文件。

### biz/
- `code.go`：定义业务错误码。

### utils/
- `file.go`：文件相关工具。
- `md.go`：Markdown 处理逻辑。
- `md_test.go`：Markdown 工具测试。
- `token.go`：token 计数能力，依赖 `tiktoken-go`。
- `epub.go`：EPUB 相关辅助能力。
- `test.md`：Markdown 工具测试输入样例。

### go.mod / go.sum
- `go.mod`：模块名 `common`，声明 Go 1.24.6。
- `go.sum`：依赖校验文件。

## 上下游关系
- 上游：`app` 引用 `common/biz` 和 `common/utils`。
- 下游：依赖 Thunder 错误处理和 `tiktoken-go`。
- 共享依赖：Go 标准库、Thunder、token 编码库。

## 何时先看这个模块
- 调整统一错误码或错误返回语义时。
- 排查 Markdown 切分、文件处理、EPUB 解析或 token 统计问题时。
- 为多个后端模块增加可复用工具函数时。

## 不要碰
- `go.sum` 由 Go 命令维护。
- `utils/test.md` 是测试样例，修改前要同步检查相关测试。
