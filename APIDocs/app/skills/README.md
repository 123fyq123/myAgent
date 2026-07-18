# skills API 全链路

## 一句话说明

`skills` 功能负责管理本地技能、GitHub 技能源和从 GitHub 安装技能，并通过 Agent 关联接口把技能挂到智能体运行链路中。当前仓库中已确认后端 API、service、repository、GORM model 和 Agent 运行时加载链路；未定位到前端 `src/api` 中对 `/api/v1/skills` 的封装。

## 归属模块

- 主模块：`app`
- 业务域：`skills`
- 相关模块：`agents`、`model`、`common`
- 文档依据：根索引 `AI_INDEX.md`、后端索引 `backend/AI_INDEX.md`、模块说明 `backend/app/AI_CONTEXT.md`、前端说明 `frontend/AI_CONTEXT.md`

## 链路总览

1. 技能管理入口是后端 `backend/app/internal/router/skills.go` 注册的 `/api/v1/skills` 路由组。
2. handler 使用 Thunder 的 `req.JsonParam`、`req.Path`、`req.GetUserIdUUID` 完成参数绑定和用户 ID 获取，再用 `res.Success` / `res.Error` 返回统一响应。
3. `backend/app/internal/skills/service.go` 承担核心逻辑：校验本地 `SKILL.md`、分页查询、更新、删除、GitHub source 管理和 Git clone 安装。
4. `backend/app/internal/skills/model.go` 是 repository 实现，通过 GORM 读写 `skills` 和 `github_sources` 表。
5. 数据结构定义在 `backend/model/skills.go`，错误码定义在 `backend/common/biz/code.go`。
6. Agent 关联技能不是 `/api/v1/skills` 下的接口，而是 `backend/app/internal/router/agents.go` 下的 `/api/v1/agents/:id/skills` 相关接口，落到 `agent_skills` 表。
7. 智能体运行时通过 `backend/app/internal/agents/service.go` 的 `buildSkills` 读取 `agent.Skills`，按 `BaseDir` 创建 Eino skill backend，并生成 `adk.ChatModelAgentMiddleware`。

## 关键文件定位

| 位置 | 作用 | 为什么相关 |
| --- | --- | --- |
| `backend/app/internal/router/skills.go` | 注册 `/api/v1/skills` 路由组 | API 入口清单 |
| `backend/app/internal/skills/handler.go` | 参数绑定、用户 ID 获取、统一响应 | HTTP handler 落点 |
| `backend/app/internal/skills/service.go` | 技能校验、CRUD、GitHub source、安装逻辑 | 核心业务逻辑 |
| `backend/app/internal/skills/repository.go` | repository 接口 | service 与 GORM 实现的边界 |
| `backend/app/internal/skills/model.go` | GORM 查询实现 | 数据读写落点 |
| `backend/app/internal/skills/req.go` | 请求 DTO | 入参字段来源 |
| `backend/app/internal/skills/res.go` | 响应 DTO | 出参字段来源 |
| `backend/model/skills.go` | `Skill`、`GitHubSource` 数据模型 | 表结构与状态枚举 |
| `backend/common/utils/md.go` | `ParseSkillMd` | 解析 `SKILL.md` 元数据 |
| `backend/common/utils/file.go` | `CopyDir` | GitHub 安装后复制技能目录 |
| `backend/app/internal/router/agents.go` | 注册 Agent-技能关联接口 | 相关链路入口 |
| `backend/app/internal/agents/service.go` | `buildSkills`、`addSkillToAgent`、`deleteSkillFromAgent` | 技能进入智能体运行时 |
| `backend/model/agents.go` | `Agent.Skills`、`AgentSkill` | 多对多关联表结构 |
| `frontend/src` | 前端源码目录 | 当前未定位到 `/api/v1/skills` 调用封装 |

## 接口与数据结构

### HTTP 接口

| 方法 | 路径 | Handler | 入参 | 出参 | 说明 |
| --- | --- | --- | --- | --- | --- |
| `POST` | `/api/v1/skills` | `CreateSkill` | `CreateSkillReq` | `SkillResponse` | 注册本地技能 |
| `POST` | `/api/v1/skills/list` | `ListSkills` | `SearchSkillReq` | `ListSkillResponse` | 分页查询当前用户技能 |
| `GET` | `/api/v1/skills/all` | `ListSkillsAll` | 无请求体 | `[]SkillResponse` | 查询当前用户全部技能 |
| `PUT` | `/api/v1/skills` | `UpdateSkill` | `UpdateSkillReq` | `SkillResponse` | 更新技能基础信息和状态 |
| `DELETE` | `/api/v1/skills/:id` | `DeleteSkill` | path: `id` | `nil` | 删除技能 |
| `POST` | `/api/v1/skills/install` | `InstallSkill` | `InstallSkillReq` | `SkillResponse` | 从 GitHub 仓库安装技能 |
| `POST` | `/api/v1/skills/sources` | `CreateGithubSources` | `CreateGithubSourceReq` | `GithubSourceResponse` | 新增 GitHub 技能源 |
| `PUT` | `/api/v1/skills/sources` | `UpdateGithubSources` | `UpdateGithubSourceReq` | `GithubSourceResponse` | 更新 GitHub 技能源 |
| `DELETE` | `/api/v1/skills/sources/:id` | `DeleteGithubSources` | path: `id` | `nil` | 删除 GitHub 技能源 |
| `POST` | `/api/v1/skills/sources/list` | `ListGithubSources` | `SearchGithubSourceReq` | `ListGithubSourceResponse` | 分页查询 GitHub 技能源 |
| `GET` | `/api/v1/skills/sources/:id` | `GetGithubSource` | path: `id` | `GithubSourceResponse` | 获取单个 GitHub 技能源 |

鉴权/中间件：源码中没有在 `skills.go` 路由文件直接挂载中间件，但 handler 必须通过 `req.GetUserIdUUID(c)` 获取用户 ID；具体用户 ID 注入来源在 Thunder 或上层中间件中，当前文档未继续展开。

### Agent 相关 HTTP 接口

| 方法 | 路径 | Handler | 入参 | 出参 | 说明 |
| --- | --- | --- | --- | --- | --- |
| `POST` | `/api/v1/agents/:id/skills` | `AddSkillToAgent` | `AddAgentSkillReq{skillIds}` | `nil` | 批量关联技能到 Agent |
| `POST` | `/api/v1/agents/:id/skills/:skillId` | `DeleteSkillFromAgent` | path: `id`、`skillId` | `nil` | 删除 Agent 与技能关联；当前路由使用 `POST`，不是 `DELETE` |

### 请求结构

| 类型 | 字段 |
| --- | --- |
| `CreateSkillReq` | `name`、`description`、`baseDir` |
| `SearchSkillReq` | `name`、`page`、`pageSize`、`status` |
| `UpdateSkillReq` | `id`、`name`、`description`、`baseDir`、`status` |
| `InstallSkillReq` | `skillId`、`sourceId`、`targetDir`、`repoUrl`、`repoPath` |
| `CreateGithubSourceReq` | `name`、`repoUrl`、`description` |
| `UpdateGithubSourceReq` | `id`、`name`、`repoUrl`、`description` |
| `SearchGithubSourceReq` | `name`、`page`、`pageSize` |
| `AddAgentSkillReq` | `skillIds` |

### 响应结构

| 类型 | 字段 |
| --- | --- |
| `SkillResponse` | `id`、`name`、`description`、`baseDir`、`status`、`creatorId`、`createdAt`、`updatedAt` |
| `ListSkillResponse` | `total`、`skills` |
| `GithubSourceResponse` | `id`、`name`、`description`、`repoUrl`、`creatorId`、`createdAt`、`updatedAt` |
| `ListGithubSourceResponse` | `total`、`sources` |

### gRPC / Proto

当前仓库中未定位到 `skills` 功能对应的 gRPC 或 proto 文件。该功能已确认是 `app` 模块内的 HTTP + 本地 service + GORM 链路。

### 数据层

| 表 | GORM model | 主要字段 | 使用位置 |
| --- | --- | --- | --- |
| `skills` | `model.Skill` | `name`、`description`、`base_dir`、`source_id`、`status`、`creator_id` | 技能 CRUD 与运行时加载 |
| `github_sources` | `model.GitHubSource` | `name`、`repo_url`、`description`、`creator_id` | GitHub 技能源 CRUD |
| `agent_skills` | `model.AgentSkill` | `agent_id`、`skill_id`、`status`、`created_at`、`updated_at` | Agent 与技能多对多关联 |

缓存 / 消息 / 其他依赖：未定位到 Redis、消息队列或缓存参与 `skills` 管理主链路。`InstallSkill` 会访问 Git 仓库并写本地文件系统。

## 详细调用链

### 1. 入口

主入口是后端 HTTP API：

- `backend/app/internal/router/skills.go` 创建 `engin.Group("/api/v1/skills")`。
- 路由组绑定技能 CRUD、GitHub source 管理和安装接口。

前端入口：

- 已检查 `frontend/src` 中 `skills`、`Skill`、`/api/v1/skills`、`skillId`、`skillIds` 等关键词。
- 当前只定位到 `frontend/src/types/a2a.ts` 中 A2A AgentCard 的 `skills` 类型字段，未定位到调用 `/api/v1/skills` 的页面或 API service。

### 2. HTTP 层

`backend/app/internal/skills/handler.go` 中每个 handler 的结构一致：

1. 使用 `req.JsonParam` 绑定 JSON body，或用 `req.Path` 绑定路径参数。
2. 使用 `req.GetUserIdUUID(c)` 获取当前用户 ID。
3. 调用 `h.service` 对应方法。
4. 成功时 `res.Success(c, data)`，失败时记录日志并 `res.Error(c, err)`。

`InstallSkill` 额外通过 `http.NewResponseController(c.Writer)` 清除写超时，避免 Git clone 和文件复制时间较长时被写 deadline 截断。

### 3. 服务层

#### 创建本地技能

1. `createSkill` 创建 5 秒 timeout。
2. `validateSkillName(req.Name, req.BaseDir)` 检查 `baseDir/name` 目录是否存在。
3. 检查该目录是否为目录。
4. 读取 `baseDir/name/SKILL.md`。
5. 调用 `utils.ParseSkillMd` 解析元数据。
6. 校验 `metadata.Name` 与请求 `name` 大小写不敏感一致。
7. `repo.findByName` 检查技能名唯一。
8. 创建 `model.Skill`，状态为 `active`，`SourceId` 为 `local`。
9. `repo.create` 写入 `skills` 表。

#### 查询技能

1. `listSkills` 对 `page` 和 `pageSize` 设置默认值：`page <= 0` 时为 1，`pageSize <= 0` 时为 10。
2. 构造 `SkillFilter{Name, Status, Limit, Offset}`。
3. `repo.list` 按 `creator_id` 查询，并可按 `name like` 与 `status` 过滤。
4. 返回 `ListSkillResponse{total, skills}`。

`listSkillsAll` 只按 `creator_id` 查询当前用户全部技能，不做分页。

#### 更新技能

1. `repo.getSkill(req.ID)` 查询技能。
2. 不存在时返回 `biz.ErrSkillNotFound`。
3. 如果请求带新名称且不同于原名称，先 `repo.findByName` 检查唯一性。
4. 按非空字段更新 `Name`、`Description`、`BaseDir`、`Status`。
5. `repo.update` 使用 `Save` 持久化。

注意：更新技能不会重新读取 `SKILL.md` 校验目录和元数据，这一点和创建本地技能不同。

#### 删除技能

1. `repo.getSkill(id)` 查询技能。
2. 不存在时返回 `biz.ErrSkillNotFound`。
3. 在事务中调用 `repo.delete(id)` 删除 `skills` 表记录。
4. 源码注释写了“删除agent关联的技能”，但当前事务中没有实际删除 `agent_skills` 的代码。

#### GitHub source 管理

1. `createGithubSources` 按名称检查唯一，然后写入 `github_sources`。
2. `updateGithubSources` 按 ID 查找，支持更新 `Name`、`Description`、`RepoUrl`，名称变化时检查唯一。
3. `deleteGithubSources` 按 ID 查找后删除。
4. `listGithubSources` 支持按 `name like` 分页查询。
5. `getGithubSource` 按 ID 获取单个 source。

#### 从 GitHub 安装技能

1. `repo.findByName(req.SkillId)` 检查技能是否已存在。
2. 构造临时目录：`targetDir/.temp/skillId`。
3. `os.RemoveAll(tempDir)` 清理旧临时目录。
4. `git.PlainClone(tempDir, false, CloneOptions{URL: req.RepoUrl, Depth: 1})` 克隆仓库。
5. `sourcePath = tempDir/repoPath`，`targetPath = targetDir/skillId`。
6. 删除旧目标目录。
7. `utils.CopyDir(sourcePath, targetPath)` 复制技能文件。
8. 删除临时目录。
9. 读取 `targetPath/SKILL.md`，调用 `utils.ParseSkillMd` 获取描述。
10. 创建 `model.Skill`，`Name` 使用 `req.SkillId`，`BaseDir` 使用 `targetPath`，`SourceId` 使用 `req.SourceId`，状态为 `active`。
11. `repo.create` 写入 `skills` 表。

注意：安装时只校验 metadata 有 `Name` 和 `Description`，但创建记录时 `Name` 使用 `req.SkillId`，没有像 `createSkill` 那样校验 metadata name 与 `skillId` 一致。

### 4. Repository / 数据实现

`backend/app/internal/skills/repository.go` 定义接口，`backend/app/internal/skills/model.go` 用 GORM 实现：

- `findByName`：`Where("name = ?", name).First(&skill)`。
- `create`：`Create(skill)`。
- `list`：`Model(&model.Skill{}).Where("creator_id = ?", userId)`，可追加 `name like` 与 `status`，再 `Count`、`Limit`、`Offset`、`Find`。
- `listAll`：`Where("creator_id = ?", userId).Find(&skills)`。
- `getSkill`：按 `id` 查询，`gorms.IsRecordNotFoundError` 时返回 `nil, nil`。
- `update`：`Save(skill)`。
- `delete`：`Delete(&model.Skill{}, id)`。
- `transaction`：`m.db.WithContext(ctx).Transaction(f)`。
- GitHub source 方法同样通过 `model.GitHubSource` 完成 CRUD 和分页。

### 5. Agent 关联与运行时加载

Agent 关联链路在 `agents` 模块：

1. `backend/app/internal/router/agents.go` 注册 `POST /api/v1/agents/:id/skills` 和 `POST /api/v1/agents/:id/skills/:skillId`。
2. `AddSkillToAgent` 绑定 path `id` 和 body `skillIds`，调用 `addSkillToAgent`。
3. `addSkillToAgent` 先用 `repo.getAgent(ctx, userID, agentId)` 确认 Agent 属于当前用户。
4. 对每个 `skillID` 调 `repo.getAgentSkill` 检查关联是否存在。
5. 不存在则创建 `model.AgentSkill{Status: "active"}`，存在则把 `Status` 重置为 `active` 并更新 `UpdatedAt`。
6. 删除关联时，`deleteSkillFromAgent` 确认 Agent 后调用 `repo.deleteAgentSkill` 删除 `agent_skills` 记录。
7. Agent 查询实现会 `Preload("Skills")`，因此运行时 `agent.Skills` 可被加载。
8. `buildSkills(agent)` 按 `Skill.BaseDir` 分组，为每个目录创建 Eino filesystem backend，再按 `Skill.Name` 创建 `skill.NewMiddleware`，最终返回 `[]adk.ChatModelAgentMiddleware`。

### 6. 返回链路

技能管理接口都通过 Thunder `res.Success` 返回：

- 单对象操作返回 `SkillResponse` 或 `GithubSourceResponse`。
- 列表接口返回 `ListSkillResponse` 或 `ListGithubSourceResponse`。
- 删除接口返回 `nil`。

错误通过 `res.Error` 返回，已确认的业务错误包括：

- `80001`：`ErrSkillNotFound`
- `80002`：`ErrSkillAlreadyExisted`
- `80003`：`ErrGithubSourceAlreadyExisted`
- `80004`：`ErrGithubSourceNotFound`

## 调试与验证

### 最小阅读路径

1. 路由入口：`backend/app/internal/router/skills.go`
2. HTTP 处理：`backend/app/internal/skills/handler.go`
3. 业务逻辑：`backend/app/internal/skills/service.go`
4. 数据读写：`backend/app/internal/skills/model.go`
5. 表结构：`backend/model/skills.go`
6. Agent 关联：`backend/app/internal/router/agents.go`、`backend/app/internal/agents/service.go`

### 最小验证步骤

启动 `backend/app` 后，可按下面路径验证。请求需要带上项目已有认证链路可识别的登录态或 token。

```bash
curl -X POST http://localhost:<port>/api/v1/skills/list \
  -H "Content-Type: application/json" \
  -d '{"page":1,"pageSize":10}'
```

```bash
curl -X POST http://localhost:<port>/api/v1/skills \
  -H "Content-Type: application/json" \
  -d '{"name":"demo-skill","baseDir":"D:/path/to/skills","description":"demo"}'
```

本地创建验证前要保证 `D:/path/to/skills/demo-skill/SKILL.md` 存在，并且 `SKILL.md` 的 metadata `name` 能与请求 `name` 匹配。

### 建议断点位置

- `backend/app/internal/router/skills.go`：确认路由是否命中。
- `backend/app/internal/skills/handler.go`：确认 JSON body、path 参数和 user ID。
- `backend/app/internal/skills/service.go` 的 `validateSkillName`：排查本地技能目录或 `SKILL.md` 解析问题。
- `backend/app/internal/skills/service.go` 的 `installSkill`：排查 Git clone、路径复制和 metadata 读取。
- `backend/app/internal/skills/model.go` 的 `list`、`create`、`delete`：确认数据库查询和写入。
- `backend/app/internal/agents/service.go` 的 `buildSkills`：确认技能是否真正进入 Agent 运行时。

## 注意点

- 当前仓库未定位到前端 `/api/v1/skills` 调用封装，技能管理可能还缺前端接入。
- `DeleteSkill` 里有删除 Agent 关联的注释，但当前代码没有实际清理 `agent_skills`，删除技能后需要关注关联表残留或外键策略。
- `UpdateSkill` 修改 `name` 或 `baseDir` 时不会重新校验 `SKILL.md`，可能写入无法被运行时加载的技能配置。
- `InstallSkill` 会删除 `targetDir/.temp/skillId` 和 `targetDir/skillId`，调用方传入 `targetDir` 前必须非常谨慎。
- `InstallSkill` 执行 Git clone，需要网络和目标仓库可访问；这是接口链路中的外部依赖。
- `Agent` 删除技能关联当前路由是 `POST /api/v1/agents/:id/skills/:skillId`，不是常见的 `DELETE` 方法。
- `buildSkills` 创建 backend 或 middleware 失败时会记录日志并继续处理其他技能，不会直接让整个 Agent 构建失败。

## 资料来源

- 未使用外部资料；本文档只依据当前仓库源码和已生成的 AI 上下文文档。
