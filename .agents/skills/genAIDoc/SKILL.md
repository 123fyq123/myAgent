---
name: genAIDoc
description: 为项目、工作区或模块生成和更新 AI 上下文文档（AI_CONTEXT.md / AI_INDEX.md）。当用户提到“为 xxx 模块生成文档”“更新 xxx 的 AI_CONTEXT.md”“生成根目录索引”“生成 AI_INDEX.md”“给整个项目生成 AI 上下文”“更新模块说明书”等请求时使用。
---

# genAIDoc · 模块文档生成器

为指定模块生成 `AI_CONTEXT.md`（模块上下文）或 `AI_INDEX.md`（仓库/工作区索引）。

文档的具体结构模板见：
- [`references/ai-context-structure.md`](references/ai-context-structure.md) — AI_CONTEXT.md 结构
- [`references/ai-index-structure.md`](references/ai-index-structure.md) — AI_INDEX.md 结构

## 触发场景

- "为 xxx 模块生成文档"
- "更新 xxx 的 AI_CONTEXT.md"
- "生成根目录索引"
- "给整个项目生成 AI 上下文"
- "更新模块说明书"

## 执行步骤

### 第一步：判断生成目标

根据用户请求先判断目标类型：

- 生成仓库或工作区索引：目标文件是 `AI_INDEX.md`。
- 生成单个模块说明：目标文件是 `AI_CONTEXT.md`。
- 生成整个项目 AI 文档：先处理各模块 `AI_CONTEXT.md`，再汇总生成根目录或工作区的 `AI_INDEX.md`。
- 增量更新文档：先从变更文件定位受影响模块，再局部更新对应文档。

### 第二步：读取对应模板

- 生成 `AI_INDEX.md` 时，读取 `references/ai-index-structure.md`。
- 生成 `AI_CONTEXT.md` 时，读取 `references/ai-context-structure.md`。
- 不要跳过模板读取；最终文档必须匹配模板章节顺序和章节名。

### 第三步：采集事实证据

生成任何文档前都必须实际扫描项目，不允许凭空编写：

- 运行 `python scripts/scan_modules.py --root <repo-or-workspace>`，获取模块清单、模块类型和已有 AI 文档路径。
- 如果目标涉及 Go 模块，运行 `python scripts/scan_go_deps.py --root <go-workspace> --module-path <module-path>`，获取依赖关系。
- 继续人工读取关键源码文件，例如 `main.go`、`go.mod`、`go.work`、`package.json`、`router/`、`api/`、`api/proto/`、`pkg/service/`、`internal/`。

### 第四步：按目标生成文档

生成 `AI_INDEX.md`：

1. 汇总仓库或工作区定位、技术栈、模块清单。
2. 为每个模块写清楚一句话定位和下一步应读的 `AI_CONTEXT.md` 路径。
3. 如存在跨模块调用链，补充典型调用链。
4. 写清楚不应该触碰的运行时产物、生成产物或缓存目录。
5. 输出一次进度完成清单，说明索引文档是否完成、模块清单是否齐全、校验是否通过。

生成 `AI_CONTEXT.md`：

1. 写清楚模块定位和主要职责。
2. 按真实目录结构说明入口文件、关键目录和关键文件。
3. 从源码 import、路由注册、RPC 注册、配置加载中确认上下游关系。
4. 写清楚什么场景下应优先阅读该模块。
5. 写清楚该模块中不应该触碰的运行时产物、生成产物或缓存目录。
6. 输出一次进度完成清单，说明模块说明是否完成、依赖分析是否完成、校验是否通过。

增量更新：

1. 使用 `git diff --name-only` 找到变更文件。
2. 根据路径前缀定位受影响模块。
3. 只更新受影响章节，不重写无关内容。
4. 保留已有的已知问题、坑点和历史说明，除非用户明确要求清理。
5. 输出一次进度完成清单，说明受影响模块、已更新章节和剩余问题。

### 第五步：校验并修正

生成或更新后必须校验：

1. 运行 `python scripts/validate_ai_docs.py --root <repo-or-workspace>`。
2. 如果脚本报告占位符、断链或路径错误，先修正文档再交付。
3. 如果校验失败来自既有文档中的历史问题，说明具体文件和行号，不把它伪装成通过。

### 第六步：输出进度完成清单

每个流程执行完后，都要输出一次进度完成清单，告诉用户当前完成进度、已完成事项、未完成事项和阻塞项。

推荐格式：

```markdown
## 进度完成清单

- 当前目标：<AI_INDEX.md / AI_CONTEXT.md / 增量更新 / 整个项目>
- 完成进度：<如 3/5、80%、已完成>
- 已完成：
  - <事项1>
  - <事项2>
- 进行中：
  - <事项>
- 未完成：
  - <事项>
- 阻塞或异常：
  - <无 / 具体问题>
```

输出要求：

- 如果流程已经全部完成，`进行中` 和 `未完成` 可写 `无`。
- 如果校验脚本报错，要把具体文件和问题写进 `阻塞或异常`。
- 如果是整个项目生成，要分别说明模块文档完成情况和索引文档完成情况。
- 如果是增量更新，要明确写出受影响模块和本次实际更新的章节。

## 三条核心规则

### 规则1：三阶段不可跳步

每个模块按顺序执行：

1. **扫描** — 读目录结构 + 读入口文件 + 读关键源文件
2. **归纳** — 提炼职责、目录作用、依赖关系、启动流程
3. **生成** — 填模板 + 质量自检

### 规则2：质量三检查

生成后必须通过：

- 无占位符（禁止 `TODO` / `[请补充]` / `待填写` / `xxx` / `...` 占位）
- 目录结构与实际一致（不能写出不存在的目录或文件）
- 文件引用路径正确（相对路径可解析）
- 可运行 `python scripts/validate_ai_docs.py --root <repo-or-workspace>` 做生成后校验；该脚本用于检查占位符、`AI_INDEX.md` / `AI_CONTEXT.md` 引用路径和索引引用关系。

### 规则3：必须严格按照模板填充

- 生成 `AI_CONTEXT.md` 必须严格遵循 [`references/ai-context-structure.md`](references/ai-context-structure.md) 的章节顺序和章节名。
- 生成 `AI_INDEX.md` 必须严格遵循 [`references/ai-index-structure.md`](references/ai-index-structure.md) 的章节顺序和章节名。
- 不得自行增删章节、不得改写章节标题、不得调整章节顺序。
- 模板里标注"按需"的章节，无内容时可省略，但不得改变其余章节。

### 规则4：文档放模块一级目录

- `AI_CONTEXT.md` 放在模块根目录
- `AI_INDEX.md` 放在仓库根目录或工作区根目录（如 `backend/ms_project/`）
- 不放代码子目录

## 模块识别策略

### 识别信号

满足任一即视为模块：

- `go.mod` — Go 模块
- `main.go` — Go 服务入口
- `package.json` — 前端模块
- `go.work` — Go 工作区（根索引级）

### 排除黑名单

`.git`、`node_modules`、`vendor`、`logs`、`.gocache`、`upload`、`dist`、`.idea`、`.vscode`、`profile_test`、`analyze`、`build`、`curl`、`t`

### 模块扫描脚本

生成 `AI_INDEX.md` 或定位目标模块前，可运行 `python scripts/scan_modules.py --root <repo-or-workspace>`。

该脚本用于扫描模块根目录、模块类型、入口标记和已有的 `AI_CONTEXT.md`，结果作为模块清单和索引入口的事实依据。

## 工作流

### 模式A：全量模式（单个模块）

输入：模块路径

步骤：

1. 扫描目录结构：`find <模块> -maxdepth 3 -type d`，排除黑名单
2. 读取入口文件（`main.go` / `index.js` 等）
3. 读取关键子目录内容（`router/`、`api/`、`pkg/service/`、`internal/` 等）
4. 如果目标是 Go 模块，运行 `python scripts/scan_go_deps.py --root <go-workspace> --module-path <module-path>`，确认 `go.mod`、内部 import、外部 import 和反向依赖
5. 按对应模板填充（`AI_CONTEXT.md` 见 [`references/ai-context-structure.md`](references/ai-context-structure.md)，`AI_INDEX.md` 见 [`references/ai-index-structure.md`](references/ai-index-structure.md)）
6. 质量自检（规则2）
7. 输出一次进度完成清单，说明该模块文档是否已完成、还有没有待修正问题。

### 模式B：增量模式

输入：`git diff --name-only`

步骤：

1. 从 diff 结果识别受影响的模块（按目录前缀聚合）
2. 只重扫受影响模块
3. 按增量规则更新对应 `AI_CONTEXT.md`
4. 更新元数据时间戳
5. 输出一次进度完成清单，说明本次受影响模块、已更新章节和剩余待处理项。

## 增量更新规则

- **只增不删**：已知问题章节、坑点保留历史记录，不因代码变化而删除
- **局部更新**：只动受影响章节，不重写整个文件
- **时间戳**：更新元数据中的更新时间

## 执行要求

- 必须先实际读目录和源文件，不能凭空生成。
- 目录结构段必须列出实际文件名，不能写"等"或省略。
- 启动流程必须按 `main.go` 的实际执行顺序写，不能臆造。
- 上下游关系必须从源码 import 确认，不能猜测。
- 引用其他模块的 `AI_CONTEXT.md` 时，路径要写对。
