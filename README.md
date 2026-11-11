# Portfolio Hub

Portfolio Hub 是一个个人 GitHub 项目展示站，用于聚合公开仓库、语言统计、提交活动、仓库 README 和 AI 生成的项目简介。项目采用前后端分离的单仓结构：前端负责页面展示和管理后台，后端负责 GitHub API 聚合、AI 摘要生成、缓存和管理接口。

## 技术栈

- 前端：Next.js 15、React 19、TypeScript、Tailwind CSS、Recharts、Framer Motion、Lucide React
- 后端：Go 1.23、Gin
- 缓存：Redis；运行中故障时熔断 30 秒并降级为进程内存，随后自动探测恢复
- 数据源：GitHub REST API、GitHub GraphQL API、OpenCode Go/Console（默认）或 OpenAI 兼容 AI API
- 包管理：npm workspaces
- 容器化：前端和后端分别提供 Dockerfile

## 功能概览

- 首页展示精选项目、语言分布、贡献活动和站点 AI 简介。
- 项目列表支持按星标、更新时间、语言和关键词筛选。
- 项目详情页展示仓库 README、语言占比、项目元数据和 AI 项目简介。
- 活动页展示 GitHub 贡献热力图，并可查看某天的 commit 详情和文件变更摘要。
- 管理后台支持登录、刷新缓存、清理缓存、生成 AI 调试摘要、编辑 `.env`、配置精选项目和隐藏项目。

## 目录结构

```text
.
├── README.md
├── package.json
├── package-lock.json
├── .env.example
└── apps
    ├── web
    │   ├── app                 # Next.js App Router 页面
    │   ├── components          # 页面组件和业务组件
    │   ├── lib                 # API 客户端、类型和站点配置
    │   ├── Dockerfile
    │   └── package.json
    └── server
        ├── cmd/server          # Go 服务入口
        ├── internal/cache      # Redis 和内存缓存封装
        ├── internal/config     # 环境变量和 .env 读写
        ├── internal/handler    # HTTP 路由处理
        ├── internal/middleware # CORS 和鉴权中间件
        ├── internal/model      # API 数据模型
        ├── internal/response   # 统一响应结构
        ├── internal/service    # GitHub 和 AI 业务逻辑
        └── Dockerfile
```

## 环境要求

- Node.js 22 或兼容版本
- npm 10 或兼容版本
- Go 1.23 或兼容版本
- Redis，开发阶段可选；未启动 Redis 时后端会使用内存缓存，运行中故障会熔断 30 秒
- GitHub Personal Access Token，建议至少具备读取公开仓库和贡献数据所需权限
- OpenCode API Key（默认 `AI_PROVIDER=opencode`）；或 OpenAI 兼容 API Key（`AI_PROVIDER=openai`）。只有使用 AI 摘要能力时必需。

## 快速开始

复制环境变量模板：

```bash
cp .env.example .env
```

编辑 `.env`，至少填写 `GITHUB_USERNAME` 和 `OPENCODE_API_KEY`。如果需要继续使用旧 OpenAI 兼容接口，将 `AI_PROVIDER` 改为 `openai` 并填写 `AI_API_KEY`。

安装前端依赖：

```bash
npm install
```

安装后端依赖：

```bash
cd apps/server
go mod download
cd ../..
```

启动后端服务：

```bash
npm run dev:server
```

启动前端服务：

```bash
npm run dev:web
```

默认访问地址：

- 前端：`http://localhost:3000`
- 后端：`http://localhost:7020`
- 健康检查：`http://localhost:7020/api/health`
- 管理后台：`http://localhost:3000/admin`

## 环境变量

项目会从当前目录或上级目录自动查找 `.env` 文件。不要提交真实 `.env`，只提交 `.env.example`。

| 变量 | 必填 | 说明 |
| --- | --- | --- |
| `GITHUB_USERNAME` | 是 | 要展示的 GitHub 用户名。 |
| `GITHUB_TOKEN` | 建议 | GitHub API Token。未配置时容易触发低额度限制，贡献活动和私有信息也可能不可用。 |
| `FEATURED_REPOS` | 否 | 首页精选项目列表，逗号分隔。 |
| `HIDDEN_REPOS` | 否 | 不展示的仓库名，逗号分隔。 |
| `AI_PROVIDER` | 否 | AI 提供方：`opencode`（默认）或 `openai`。不会跨提供方自动回退。 |
| `OPENCODE_API_KEY` | opencode 必填 | OpenCode Go/Console 的 API Key，通常以 `oc_sk_` 开头。 |
| `OPENCODE_BASE_URL` | 否 | OpenCode OpenAI 兼容接口地址，默认 `https://opencode.ai/inference/openai/v1`。 |
| `AI_API_KEY` | openai 必填 | 旧 OpenAI 兼容接口的 API Key。 |
| `AI_BASE_URL` | 否 | 旧 OpenAI 兼容接口地址，默认 `https://api.openai.com/v1`。 |
| `AI_MODEL` | 否 | AI 摘要使用的模型名；`opencode` 默认 `deepseek-v4.1-flash`，`openai` 默认 `gpt-4o-mini`。 |
| `BLOG_URL` | 否 | 后端使用的博客链接。 |
| `SITE_NAME` | 否 | 后端站点名称。 |
| `SITE_DESCRIPTION` | 否 | 后端站点描述。 |
| `ADMIN_PASSWORD` | 是 | 管理后台登录密码，生产环境必须修改默认值。 |
| `JWT_SECRET` | 是 | 管理后台 JWT 签名密钥，生产环境必须使用足够长的随机值。 |
| `REDIS_ADDR` | 否 | Redis 地址，例如 `localhost:6379`。为空时禁用 Redis；连接失败时进入 30 秒熔断并降级到内存。 |
| `REDIS_PASSWORD` | 否 | Redis 密码。 |
| `REDIS_DB` | 否 | Redis 数据库编号。 |
| `SERVER_PORT` | 否 | 后端监听端口，默认 `7020`。 |
| `WEB_ORIGIN` | 否 | CORS 允许的前端来源，默认 `http://localhost:3000`。 |
| `API_BASE_URL` | 否 | 前端服务端重写 `/api` 的目标地址，默认 `http://localhost:7020`；Docker 中为 `http://server:7020`。 |
| `NEXT_PUBLIC_API_BASE_URL` | 否 | 浏览器访问 API 的可选绝对地址；留空表示使用同源 `/api`。 |
| `NEXT_PUBLIC_BLOG_URL` | 否 | 前端展示的博客链接。 |
| `NEXT_PUBLIC_SITE_NAME` | 否 | 前端站点名称。 |
| `NEXT_PUBLIC_SITE_DESCRIPTION` | 否 | 前端站点描述。 |
| `NEXT_PUBLIC_GITHUB_URL` | 否 | 前端导航中的 GitHub 链接。 |

生产环境至少需要修改 `ADMIN_PASSWORD`、`JWT_SECRET`，并确保所有密钥只通过环境变量或密钥管理服务注入。

## 常用命令

在仓库根目录运行：

| 命令 | 说明 |
| --- | --- |
| `npm run dev:web` | 启动 Next.js 开发服务器。 |
| `npm run build:web` | 构建前端生产产物。 |
| `npm run lint:web` | 运行前端 lint。 |
| `npm run dev:server` | 启动 Go API 服务。 |
| `npm run test:server` | 运行 Go 后端测试。 |
| `npm run test:web` | 运行前端 Vitest 测试。 |
| `npm run fmt:server` | 格式化 Go 后端代码。 |

也可以进入子目录执行原生命令：

```bash
cd apps/web
npm run dev
npm run build
npm run lint
```

```bash
cd apps/server
go run ./cmd/server
go test ./...
gofmt -w ./cmd ./internal
```

## 前端开发说明

前端位于 `apps/web`，使用 Next.js App Router。

主要页面：

- `app/page.tsx`：首页。
- `app/projects/page.tsx`：项目列表。
- `app/projects/[name]/page.tsx`：项目详情。
- `app/activity/page.tsx`：贡献活动。
- `app/activity/[date]/page.tsx`：指定日期的活动详情。
- `app/about/page.tsx`：关于页。
- `app/admin/page.tsx`：管理后台。

数据访问集中在 `apps/web/lib`：

- `api.ts`：公开页面使用的 API 客户端，内置 fallback 数据，后端不可用时页面仍可渲染。
- `admin-api.ts`：管理后台 API 客户端，失败时抛出错误并展示给后台 UI。
- `types.ts`：前端 API 类型。
- `site-config.ts`：前端公开站点配置。

开发新页面时优先复用 `components` 下已有布局、卡片、图表和动效组件；新增接口调用应放在 `lib/api.ts` 或 `lib/admin-api.ts`，并同步补充 `lib/types.ts`。

## 后端开发说明

后端位于 `apps/server`，入口为 `cmd/server/main.go`。服务启动时会加载配置、初始化缓存、注册中间件和路由。

核心模块：

- `internal/config`：读取环境变量，自动加载 `.env`，支持管理后台读写 `.env`。
- `internal/cache`：封装 Redis JSON 缓存、列表记录和按 pattern 删除；Redis 不可用时使用内存实现，运行中故障会触发 30 秒熔断并自动探测恢复。
- `internal/handler`：注册 HTTP 路由并处理请求参数、响应和错误。
- `internal/service/github_service.go`：读取 GitHub 仓库、README、语言统计、贡献活动和 commit 详情。
- `internal/service/ai_service.go`：基于仓库详情和文件信息生成项目摘要、站点摘要及调试日志。
- `internal/middleware`：CORS 和管理后台 JWT 鉴权。

统一响应结构：

```json
{
  "success": true,
  "data": {}
}
```

失败响应：

```json
{
  "success": false,
  "error": "error message"
}
```

## API 概览

公开接口：

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `GET` | `/api/health` | 服务和缓存状态检查。 |
| `GET` | `/api/github/repos` | 项目列表，支持 `sort`、`language`、`keyword` 查询参数。 |
| `GET` | `/api/github/repos/:name` | 项目详情和 README。 |
| `GET` | `/api/github/languages` | 汇总语言统计。 |
| `GET` | `/api/github/activity` | GitHub 贡献活动。 |
| `GET` | `/api/github/activity/:date` | 指定日期 commit 详情，日期格式为 `YYYY-MM-DD`。 |
| `POST` | `/api/ai/project-summary` | 生成或读取项目 AI 摘要。 |
| `POST` | `/api/ai/site-summary` | 生成或读取站点 AI 摘要。 |

AI 摘要请求示例：

```json
{
  "repoName": "portfolio-hub",
  "forceRefresh": false
}
```

管理接口：

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `POST` | `/api/admin/login` | 使用 `ADMIN_PASSWORD` 登录，返回 JWT。 |
| `GET` | `/api/admin/cache/records` | 读取最近缓存操作记录。 |
| `POST` | `/api/admin/cache/refresh/repos` | 刷新项目列表缓存。 |
| `POST` | `/api/admin/cache/refresh/languages` | 刷新语言统计缓存。 |
| `POST` | `/api/admin/cache/refresh/activity` | 刷新贡献活动缓存。 |
| `POST` | `/api/admin/cache/refresh/all` | 刷新核心缓存并清理派生缓存。 |
| `POST` | `/api/admin/cache/clear/repo-details` | 清理项目详情和 README 缓存。 |
| `POST` | `/api/admin/cache/clear/repo-browser` | 清理仓库目录和文件内容缓存。 |
| `POST` | `/api/admin/cache/clear/ai-projects` | 清理项目 AI 摘要缓存。 |
| `POST` | `/api/admin/cache/clear/ai-site` | 清理站点 AI 摘要缓存。 |
| `POST` | `/api/admin/cache/clear/ai` | 清理全部 AI 缓存。 |
| `POST` | `/api/admin/ai/project-summary` | 生成项目 AI 调试摘要。 |
| `GET` | `/api/admin/env` | 读取 `.env`。 |
| `PUT` | `/api/admin/env` | 保存 `.env`。 |
| `GET` | `/api/admin/featured` | 读取精选项目。 |
| `POST` | `/api/admin/featured` | 保存精选项目。 |
| `GET` | `/api/admin/hidden-projects` | 读取隐藏项目。 |
| `POST` | `/api/admin/hidden-projects` | 保存隐藏项目。 |

除登录接口外，管理接口都需要请求头：

```text
Authorization: Bearer <token>
```

登录失败按客户端 IP 限流：5 分钟内最多允许 5 次失败尝试，超限返回 `429` 和 `Retry-After`。Token 与当前 `ADMIN_PASSWORD` 绑定，修改管理员密码后，已签发的 token 会立即失效，需要重新登录。

## 缓存策略

后端会优先连接 Redis；如果 `REDIS_ADDR` 为空，则只使用进程内存缓存。Redis 连接失败或运行中出错时，缓存层会进入 30 秒熔断：期间读写走进程内存，pattern 删除会记录并在恢复后补执行；30 秒后下一次操作会重新探测 Redis，成功后自动恢复。健康检查的 `redis` 字段会返回 `ok`、`degraded` 或 `disabled`。内存缓存适合本地开发，不适合多实例生产部署。

当前缓存覆盖的主要内容：

- GitHub 仓库列表
- 仓库详情和 README
- 仓库语言统计
- GitHub 贡献活动和 commit 详情
- AI 项目摘要和站点摘要
- 管理后台缓存操作记录

更新 `.env`、修改精选项目或隐藏项目后，相关派生缓存会被清理。需要立即同步 GitHub 数据时，可以通过管理后台或管理接口刷新缓存。

## 测试和质量检查

前端检查：

```bash
npm run lint:web
npm run test:web
npm run build:web
```

后端检查：

```bash
npm run fmt:server
npm run test:server
```

提交前建议至少运行后端测试、前端测试和前端构建。涉及 UI 或数据展示逻辑时，还应手动打开首页、项目页、活动页和管理后台做一次基本回归。

## Docker 构建

后端镜像：

```bash
docker build -f apps/server/Dockerfile -t portfolio-hub-server .
```

前端镜像：

```bash
docker build -f apps/web/Dockerfile -t portfolio-hub-web .
```

推荐使用 `deploy/docker-compose.yaml` 启动完整栈：

```bash
docker compose --env-file .env -f deploy/docker-compose.yaml up --build
```

Compose 会启动 nginx、前端、后端和 Redis。浏览器和 nginx 使用同源 `/api`，nginx 将 `/api/` 转发到 `server:7020`，其余请求转发到 `web:3000`；只有 nginx 暴露 80 端口。`--env-file .env` 让 Compose 把前端的 `NEXT_PUBLIC_*` 作为构建参数注入镜像；修改这些公开变量后需要重新执行 `up --build`。后端容器会挂载根目录 `.env`，管理后台的 `.env` 编辑才会落盘并在重启后保留。前端容器不再接收完整的 `.env`，避免把 GitHub、AI 和管理后台密钥暴露给前端镜像。
