# thunder

## 一句话先懂

在本项目里，`thunder` 不是替代 Gin 的新 Web 框架，而是对 Gin 的一层项目基础设施封装。它负责读取 `etc/config.yml`、创建 `gin.Engine`、挂载通用中间件、批量注册路由、初始化日志/数据库/JWT 等能力，让 `app` 和 `mcp-server` 两个 Go 服务用同一套启动方式运行。

## 学习目标

- 理解本项目为什么看不到很多 `gin.Default()`，但后端仍然是 Gin。
- 能从 `main.go` 追到 `thunder/server.NewServer`、`RegisterRouters` 和业务路由。
- 能判断跨域、鉴权、端口、Gin mode 等配置应该从哪里看、在哪里生效。

## 通用背景

Gin 本身解决的是 HTTP 路由、中间件、请求上下文和响应处理。`thunder` 在 Gin 之上补了一层项目约定：

- `config`：用 Viper 从当前服务工作目录的 `etc/config.yml` 读取配置。
- `server`：创建 Gin 引擎，按配置加载中间件，并通过 `http.Server` 启动服务。
- `midd`：提供 CORS、Auth、Cache 等通用中间件。
- `req` / `res`：封装请求参数读取和响应输出。
- `database` / `jwt` / `logs`：提供数据库、令牌、日志等基础能力。

所以阅读本项目后端时，可以把 `thunder` 理解成“Gin 的启动器和基础设施包”。

## 通用操作步骤

### 常用命令或操作

```bash
# 查项目哪里直接使用 thunder
rg -n "github.com/mszlu521/thunder|server.NewServer|RegisterRouters" backend

# 查项目哪里仍然直接使用 Gin
rg -n "github.com/gin-gonic/gin|gin.Context|gin.Engine" backend

# 查 thunder 模块里的服务启动逻辑
rg -n "func NewServer|func \\(s \\*Server\\) Start|type IRouter|UseCustomMidd" D:/Golang/WorkSpace/pkg/mod/github.com/mszlu521/thunder@v1.0.3

# 查当前服务的端口、Gin 模式、跨域和鉴权配置
rg -n "server:|mode:|host:|port:|cros:|auth:|isAuth|needLogins|ignores" backend/app/etc/config.yml backend/mcp-server/etc/config.yml
```

### 标准流程

1. 先看服务入口：`main.go` 通常执行 `config.Init()`、`logs.Init()`、`server.NewServer()`、`inits.Init()`、`s.Start()`。
2. 再看初始化：`internal/inits/inits.go` 会初始化依赖，并调用 `s.RegisterRouters(...)`。
3. 最后看路由：每个 router 实现 `Register(engine *gin.Engine)`，在里面创建 Gin 路由组并绑定 handler。

## 在本项目中的落点

| 位置 | 作用 | 为什么相关 |
| --- | --- | --- |
| `backend/app/main.go` | 主业务 API 服务入口 | 通过 `thunder/config` 和 `thunder/server` 启动 8888 服务 |
| `backend/app/internal/inits/inits.go` | app 初始化依赖和路由 | 初始化 Postgres、Redis、JWT、系统工具，然后注册业务路由 |
| `backend/app/internal/router/*.go` | app 的 Gin 路由 | 每个路由结构都实现 `Register(engine *gin.Engine)` |
| `backend/mcp-server/main.go` | 独立 MCP 服务入口 | 同样通过 `thunder/server` 启动服务 |
| `backend/mcp-server/internal/router/mcp.go` | MCP SSE 路由 | 直接使用 `gin.Engine` 注册 `/sse` 和 `/message` |
| `D:/Golang/WorkSpace/pkg/mod/github.com/mszlu521/thunder@v1.0.3/server/server.go` | thunder 服务封装 | 内部调用 `gin.Default()` 并启动 `http.Server` |
| `D:/Golang/WorkSpace/pkg/mod/github.com/mszlu521/thunder@v1.0.3/server/router.go` | 路由接口和中间件挂载 | 定义 `IRouter`，并按配置挂载 CORS/Auth/Cache |
| `D:/Golang/WorkSpace/pkg/mod/github.com/mszlu521/thunder@v1.0.3/midd/cros.go` | CORS 中间件 | 处理跨域 Origin、OPTIONS 预检和响应头 |

## 调用链或数据流

以 `backend/app` 为例：

1. `backend/app/main.go` 调用 `config.Init()`，`thunder` 从当前服务目录的 `etc/config.yml` 读取配置。
2. `main.go` 调用 `server.NewServer(conf)`。
3. `thunder/server.NewServer` 设置 Gin mode，创建 `gin.Default()`，再调用 `UseCustomMidd(conf, engine)`。
4. `UseCustomMidd` 根据配置决定是否挂载 CORS、Auth、Cache 中间件。
5. `backend/app/internal/inits/inits.go` 初始化 Postgres、Redis、JWT，并调用 `s.RegisterRouters(...)`。
6. `RegisterRouters` 先注册事件，再遍历业务 router，把它们挂到同一个 `gin.Engine` 上。
7. 请求进入 Gin 后，先经过 thunder 中间件，再进入具体业务 handler，例如 `auths.Handler.Login`、`agents.Handler.AgentMessage`。
8. `s.Start()` 用配置里的 host、port、readTimeout、writeTimeout 创建 `http.Server`，并支持 SIGINT/SIGTERM 优雅关闭。

`backend/mcp-server` 的链路类似，只是它注册的是 MCP SSE 路由，默认服务配置指向 7777 端口。

## 核心概念

### Server

`thunder/server.Server` 里最关键的是三个字段：`Engine *gin.Engine`、`httpServer *http.Server`、`conf *config.Config`。这说明它没有隐藏 Gin，而是把 Gin 引擎作为核心对象包起来。

### IRouter

`IRouter` 接口要求实现：

```go
Register(engine *gin.Engine)
```

本项目的 `AuthRouter`、`AgentRouter`、`KnowledgeBaseRouter`、`ToolRouter` 等都遵循这个约定。新增一组 API 时，通常新建一个 router，实现 `Register`，然后在 `internal/inits/inits.go` 里传给 `s.RegisterRouters(...)`。

### 配置驱动中间件

`UseCustomMidd` 会检查配置：

- `server.cros` 非空：挂载 `midd.Cors`。
- `auth.isAuth` 为 true：挂载 `midd.Auth`。
- `cache.needCache` 非空：挂载 `midd.Cache`。

这意味着跨域和鉴权问题，不能只看 handler，也要看 `etc/config.yml` 和 thunder 的中间件逻辑。

### CORS

你当前打开的 `midd/cros.go` 会读取请求头 `Origin`，再和 `conf.Cros` 里的允许来源匹配。它支持三类规则：

- `*`：允许所有来源。
- 完整域名匹配：例如只允许某个前端地址。
- 子域通配：例如 `*.example.com`。

注意这个实现最后无条件设置了 `Access-Control-Allow-Credentials: true`。如果 `Access-Control-Allow-Origin` 是 `*`，这在浏览器跨域凭据场景里容易踩坑；如果后续遇到“前端跨域但 Postman 正常”，优先检查这里。

### Auth

`midd/auth.go` 会先检查 `auth.ignores`，命中则跳过鉴权。否则读取 `Authorization` 请求头，支持去掉 `Bearer ` 前缀，再用 `jwt.ParseToken` 解析，并把 `userId` 写入 Gin context。

这里还有一个容易误解的点：`reject` 里如果请求路径匹配 `needLogins`，会直接 `ctx.Next()`，不返回 401。也就是说 `needLogins` 更像“允许未登录继续走业务逻辑的路径”，不是“必须登录”的路径名。

## 本项目实践路线

```bash
# 1. 从 app 入口开始读
Get-Content backend/app/main.go

# 2. 看 app 注册了哪些 thunder 路由
Get-Content backend/app/internal/inits/inits.go

# 3. 挑一个路由看 Gin 注册方式
Get-Content backend/app/internal/router/auths.go

# 4. 对照 thunder 的路由接口
Get-Content D:/Golang/WorkSpace/pkg/mod/github.com/mszlu521/thunder@v1.0.3/server/router.go

# 5. 对照 thunder 的 Gin 创建和启动逻辑
Get-Content D:/Golang/WorkSpace/pkg/mod/github.com/mszlu521/thunder@v1.0.3/server/server.go

# 6. 如果是跨域问题，看 CORS 中间件和 config.yml
Get-Content D:/Golang/WorkSpace/pkg/mod/github.com/mszlu521/thunder@v1.0.3/midd/cros.go
```

一个小练习：从前端登录接口出发，追 `POST /api/v1/auth/login`。路线应该是：

1. `backend/app/internal/router/auths.go` 注册 `/api/v1/auth/login`。
2. 请求进入 `backend/app/internal/auths/handler.go` 的 `Login`。
3. handler 调用 service 完成登录。
4. 返回响应时使用 thunder 的 `req` / `res` 工具辅助处理参数和输出。

## 注意点

- 本项目后端仍然使用 Gin。`thunder` 内部调用 `gin.Default()`，业务 handler 也使用 `*gin.Context`。
- `thunder` 位于 Go module 缓存目录，不属于本仓库源码。直接改 `D:/Golang/WorkSpace/pkg/mod/...` 通常不可靠，依赖更新后可能丢失；如果要长期修改，应 fork 或 replace 到本地模块。
- `app` 和 `mcp-server` 都依赖各自运行目录下的 `etc/config.yml`。同名配置项在两个服务里可能端口不同、超时不同。
- 配置文件可能包含数据库、邮件、JWT 等敏感参数。学习和排查时只引用字段含义，不要把真实密码、密钥写进文档或提交。
- `cros` 这个命名应理解为 CORS，属于历史命名差异，不影响运行。

