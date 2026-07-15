# frontend 说明

## 模块定位
`frontend` 是 MyAgent 的 Vue 3 前端应用，承载登录、首页、智能体、工作流、知识库、模型、工具、模板、云存储、MCP 市场和订阅等操作界面。

## 主要职责
- 提供用户可操作的管理与执行页面。
- 通过 Vue Router 管理认证页面和主应用页面。
- 通过 Pinia 保存用户、智能体、工作流、知识库、工具和模板状态。
- 通过 `src/api` 调用后端 `app` 服务。
- 提供可视化工作流编辑器和节点属性配置界面。

## 目录结构

### main.ts / 入口文件
- `createApp(App)` 创建 Vue 应用。
- `createPinia()` 创建状态管理实例。
- `pinia.use(piniaPluginPersistedstate)` 启用持久化状态。
- `app.use(router)` 注册路由。
- `app.use(ElementPlus, { locale: zhCn })` 注册中文 Element Plus。
- `app.mount('#app')` 挂载应用。

### src/router/
- `index.ts`：定义登录、注册、找回密码、首页、智能体、模型、知识库、工具、模板、云存储、订阅、工作流、MCP 市场和智能体市场路由，并通过路由守卫检查认证状态。

### src/api/
- `http.ts`：HTTP 客户端封装。
- `auth.ts`、`agentService.ts`、`workflow.ts`、`modelService.ts`、`knowledgeBaseService.ts`、`toolService.ts`、`templateService.ts`、`settingsService.ts`、`subscriptionService.ts`、`a2aService.ts`、`cloudStorageService.ts`、`agentCommunicationService.ts`、`nodeService.ts`、`code.ts`：按业务域封装后端接口。

### src/views/
- `Login.vue`、`Register.vue`、`ForgotPassword.vue`、`Unauthorized.vue`：认证相关页面。
- `HomeView.vue`、`AgentManagement.vue`、`AgentExecute.vue`、`WorkflowLayout.vue`、`KnowledgeManagementView.vue`、`ModelManagement.vue`、`ToolManagementView.vue`、`TemplateManagementView.vue`、`CloudStorageView.vue`、`SubscriptionView.vue`、`McpMarket.vue`、`AgentMarket.vue`、`Settings.vue`、`Profile.vue`、`TaskList.vue`：主应用页面。

### src/components/
- `WorkflowEditor/`：工作流管理、画布、节点面板、节点属性、导入导出、执行预览和节点组件。
- `agent/`：智能体会话、上下文和信息面板。
- `knowledge/`：知识库列表、详情、搜索、表单和上传组件。
- `tools/`、`template/`、`storage/`、`dialogs/`、`layout/`、`common/`：工具、模板、存储、弹窗、布局和公共组件。

### src/stores/
- `user.ts`、`agent.ts`、`agentStore.ts`、`workflow.ts`、`knowledgeBaseStore.ts`、`toolStore.ts`、`templateStore.ts`：Pinia 状态模块。

### src/types/
- `agent.ts`、`workflow.ts`、`nodes.ts`、`node.ts`、`model.ts`、`knowledgeBase.ts`、`tool.ts`、`template.ts`、`settings.ts`、`cloudStorage.ts`、`auth.ts`、`a2a.ts`、`editor.ts`、`user.ts`：前端类型定义。

### src/utils/
- `WorkflowExecutor.ts`、`nodeFactory.ts`、`promptHighlighter.ts`、`md.ts`、`htmlUtils.ts`、`date.ts`、`throttle.ts`：工作流执行、节点构建、提示词高亮、Markdown、HTML、日期和节流工具。

## 上下游关系
- 上游：浏览器用户和开发者。
- 下游：通过 `src/api` 请求后端 `backend/app` 的 HTTP 接口。
- 共享依赖：Vue 3、Vue Router、Pinia、Element Plus、Vue Flow、Milkdown、WangEditor、Axios、D3、Marked、Markdown-it。

## 何时先看这个模块
- 修改页面、路由、前端接口调用或 Pinia 状态时。
- 排查登录跳转、工作流编辑器、智能体执行、知识库管理、工具管理或模型配置界面问题时。
- 调整前后端类型契约或新增业务页面时。

## 不要碰
- `node_modules/`、`dist/`、`.vite/` 和构建缓存不属于源码文档维护范围。
- `pnpm-lock.yaml` 由包管理器维护。
- `.vscode/` 是编辑器配置目录，除非明确调整项目开发体验，不应改动。
