# model 说明

## 模块定位
`model` 是后端数据模型库，集中定义用户、智能体、模型、工具、订阅、技能、知识库和工作流等 GORM 模型。

## 主要职责
- 提供业务实体的 GORM 结构体。
- 封装 JSON 字段、基础字段和数据库序列化逻辑。
- 为 `app` 的 repository/service 和 `core` 的 AI 能力提供共享数据结构。

## 目录结构

### main.go / 入口文件
- 本模块没有 `main.go` 文件。

### 根目录模型文件
- `base.go`：基础模型字段或通用结构。
- `users.go`：用户模型。
- `agents.go`：智能体模型。
- `workflows.go`：工作流模型。
- `llms.go`：模型配置与提供商模型。
- `tools.go`：工具模型。
- `skills.go`：技能模型。
- `knowledges.go`：知识库模型。
- `subscriptions.go`：订阅模型。
- `market.go`：市场相关模型。

### go.mod
- `go.mod`：模块名 `model`，声明 Go 1.24。

## 上下游关系
- 上游：`app` 的业务模块读写这些模型；`core` 读取部分模型配置。
- 下游：依赖 GORM、Thunder AI 结构、Eino schema 和部分 embedding 配置类型。
- 共享依赖：PostgreSQL 数据库表结构与 JSON 序列化格式。

## 何时先看这个模块
- 修改数据库字段、GORM 标签、JSON 字段或模型关联时。
- 排查 repository 查询和服务层对象映射问题时。
- 新增业务实体或扩展智能体、工具、知识库、订阅字段时。

## 不要碰
- 模型字段改动会影响数据库结构和已有数据，变更前要同步检查 repository、handler 和前端类型。
- 不要在本模块放业务流程逻辑，业务行为应留在 `app/internal` 或 `core/ai`。
