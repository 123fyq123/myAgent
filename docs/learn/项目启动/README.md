# 最简本地启动

## 1. 启动 Docker 依赖

```powershell
cd D:\MyAgent\backend\docker
docker compose up -d postgres redis elasticsearch etcd minio milvus
```

> Compose 所需的 `ROOT` 和数据库/Elasticsearch 密码等环境变量，先按本机配置准备好；不要提交真实密码。

### 重启 Docker 依赖

```powershell
cd D:\MyAgent\backend\docker

# 重启全部基础依赖
docker compose restart

# 只重启 PostgreSQL（其他服务同理）
docker compose restart postgres

# 查看容器状态
docker compose ps
```

## 2. 启动后端 API

```powershell
cd D:\MyAgent\backend\app
go run .
```

必须在 `backend/app` 目录启动。配置文件路径是相对路径；若从 `backend` 执行 `go run ./app`，程序会找错 `etc/config.yml`。

验证：

```powershell
Invoke-WebRequest http://127.0.0.1:8888/health
```

## 3. 启动前端

```powershell
cd D:\MyAgent\frontend
corepack pnpm dev
```

浏览器访问 <http://127.0.0.1:5173>。前端会将 `/api` 请求转发到后端 `127.0.0.1:8888`。

