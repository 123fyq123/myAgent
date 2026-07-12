# common 说明

## 模块定位
`common` 是主 API 服务使用的轻量通用库，集中放置业务错误码和文本处理工具。

## 主要职责
- 定义应用可复用的业务错误码。
- 提供 Markdown 标题提取、按标题切分和按长度窗口切分文本的函数。
- 使用 tiktoken 计算输入文本 token 数量。

## 目录结构

### go.mod / 模块文件
- 声明 `common` 模块及其依赖，当前源码直接使用 Thunder 错误包和 tiktoken。

### biz/
- `code.go`：定义业务层错误码。

### utils/
- `md.go`：实现 Markdown 标题提取、标题切分、定长切分和滑动窗口切分。
- `token.go`：实现 token 数量计算。
- `md_test.go`：覆盖 Markdown 切分逻辑。
- `test.md`：Markdown 切分测试数据。

## 上下游关系
- 上游：`app` 直接导入 `common/biz` 与 `common/utils`。
- 下游：依赖 Thunder 错误包和 tiktoken 库。
- 共享依赖：不依赖工作区内其他模块。

## 何时先看这个模块
- 需要调整业务错误码或统一错误表达时。
- 需要修改知识库文档的 Markdown 分割、滑动窗口逻辑或 token 统计时。

## 不要碰
- `go.sum` 由 Go 模块工具维护。
