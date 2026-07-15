# AI_INDEX

## 作用
本文件是 MyAgent 仓库的 AI 导航入口。处理前端页面、后端服务、智能体工作流、MCP、A2A 或知识库能力时，先从这里确认模块边界，再进入对应模块的 `AI_CONTEXT.md`。

## 技术栈

### 开发语言
- Go 1.24 到 1.25；后端通过 `backend/go.work` 组织多个 Go 模块。
- TypeScript 5.1 与 Vue 3；前端包名为 `faber-ai`。

### 框架与运行时
- 后端使用 Gin、Thunder、CloudWeGo Eino、Hertz、mcp-go 与 GORM。
- 前端使用 Vite、Vue Router、Pinia、Element Plus、Vue Flow、Milkdown 与 WangEditor。

### 数据与中间件
- PostgreSQL 由 Thunder 数据库组件与 GORM 使用。
- Redis 用于缓存与运行时依赖。
- Elasticsearch 与 Milvus 相关依赖用于知识库检索、向量索引和文档搜索。

### 基础设施
- `backend/k8s/` 保存 Kubernetes 部署清单。
- `backend/docker/` 保存容器与本地配置相关文件。
- 后端服务从各自的 `etc/config.yml` 读取运行配置。

## 模块清单

### backend
- Go 工作区，包含主业务 API、AI 能力库、MCP 服务、A2A 服务、公共工具、数据模型和压测脚本。
- 下一步看：`backend/AI_INDEX.md`

### frontend
- Vue 3 前端应用，提供登录、智能体管理、工作流编辑、知识库、工具、模型配置、MCP 市场和订阅等界面。
- 下一步看：`frontend/AI_CONTEXT.md`

### docs
- 项目学习笔记目录，目前包含 `docs/learn/thunder/README.md`。
- 下一步看：`docs/learn/thunder/README.md`

## 典型调用链
1. 用户在 `frontend/src/views` 或 `frontend/src/components` 中操作页面。
2. 前端通过 `frontend/src/api` 的 Axios 封装调用后端 HTTP 接口。
3. 后端 `backend/app/internal/router` 将请求分发到对应业务模块的 handler、service 和 repository。
4. 业务服务读取 `backend/model` 中的 GORM 模型，并按需调用 `backend/core/ai` 的模型、工具、知识库、MCP 和工作流能力。
5. 外部 MCP 或 A2A 集成分别通过 `backend/mcp-server` 与 `backend/a2a-server` 暴露独立协议入口。

## 不要碰
- `.git/`、`.agents/`、`.codex/` 是工具或仓库元数据目录，不属于业务代码维护范围。
- `frontend/node_modules/`、`frontend/dist/`、Go 构建缓存、日志目录和临时上传目录不应进入 AI 文档或人工维护。
- `go.sum`、`pnpm-lock.yaml` 应由依赖管理命令维护，不应手工改写。
