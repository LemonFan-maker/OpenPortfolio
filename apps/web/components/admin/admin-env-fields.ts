import { Activity, Bot, Database, Github, Globe2, KeyRound, Server } from "lucide-react";

export type EnvGroupDefinition = {
  description: string;
  icon: typeof Database;
  id: string;
  keys: string[];
  title: string;
};

export const envGroupDefinitions: EnvGroupDefinition[] = [
  {
    description: "GitHub 账号、Token 以及项目展示名单。",
    icon: Github,
    id: "github",
    keys: ["GITHUB_USERNAME", "GITHUB_TOKEN", "FEATURED_REPOS", "HIDDEN_REPOS"],
    title: "GitHub 数据",
  },
  {
    description: "用于生成项目简介和个人履历的大模型接口。",
    icon: Bot,
    id: "ai",
    keys: ["AI_PROVIDER", "OPENCODE_API_KEY", "OPENCODE_BASE_URL", "AI_API_KEY", "AI_BASE_URL", "AI_MODEL", "AI_PUBLIC_PER_MINUTE", "AI_PUBLIC_GLOBAL_PER_MINUTE", "AI_PUBLIC_MAX_CONCURRENCY"],
    title: "AI 生成",
  },
  {
    description: "站点名称、简介和博客入口。",
    icon: Globe2,
    id: "site",
    keys: ["BLOG_URL", "SITE_NAME", "SITE_DESCRIPTION"],
    title: "站点信息",
  },
  {
    description: "后台登录口令和 JWT 签名密钥。",
    icon: KeyRound,
    id: "admin",
    keys: ["ADMIN_PASSWORD", "JWT_SECRET", "ADMIN_COOKIE_SECURE"],
    title: "后台安全",
  },
  {
    description: "Redis 缓存连接信息。",
    icon: Database,
    id: "redis",
    keys: ["REDIS_ADDR", "REDIS_PASSWORD", "REDIS_DB"],
    title: "Redis 缓存",
  },
  {
    description: "Go API 服务端口和跨域来源。",
    icon: Server,
    id: "server",
    keys: ["SERVER_PORT", "WEB_ORIGIN", "TRUSTED_PROXIES"],
    title: "后端服务",
  },
  {
    description: "浏览器可见的前端公开配置。",
    icon: Activity,
    id: "frontend",
    keys: ["NEXT_PUBLIC_API_BASE_URL", "NEXT_PUBLIC_BLOG_URL", "NEXT_PUBLIC_GITHUB_URL", "NEXT_PUBLIC_SITE_NAME", "NEXT_PUBLIC_SITE_DESCRIPTION"],
    title: "前端公开参数",
  },
];

export const envFieldLabels: Record<string, { label: string; hint: string }> = {
  GITHUB_USERNAME: { label: "GitHub 用户名", hint: "用于拉取公开仓库、语言统计和 README。" },
  GITHUB_TOKEN: { label: "GitHub Token", hint: "用于贡献记录和提高 API 限额。" },
  FEATURED_REPOS: { label: "精选项目", hint: "英文逗号分隔，首页会优先按这个顺序展示。" },
  HIDDEN_REPOS: { label: "隐藏项目", hint: "英文逗号分隔，不在前台展示这些仓库。" },
  AI_PROVIDER: { label: "AI 提供方", hint: "opencode（默认）或 openai；切换后需重启后端。" },
  OPENCODE_API_KEY: { label: "OpenCode API Key", hint: "OpenCode Go/Console 的 oc_sk_ 密钥，仅后端使用。" },
  OPENCODE_BASE_URL: { label: "OpenCode Base URL", hint: "Go 订阅默认 https://opencode.ai/inference/go/openai/v1；不要使用按 Console 余额计费的 /inference/openai/v1。" },
  AI_API_KEY: { label: "OpenAI API Key（回退）", hint: "仅 AI_PROVIDER=openai 时使用，不会暴露给前端。" },
  AI_BASE_URL: { label: "OpenAI Base URL（回退）", hint: "旧 OpenAI 兼容接口地址。" },
  AI_MODEL: { label: "AI 模型", hint: "openai 默认 gpt-4o-mini；opencode 默认 deepseek-v4.1-flash。" },
  AI_PUBLIC_PER_MINUTE: { label: "AI 单 IP 限额", hint: "每个客户端 IP 每分钟允许的公开 AI 请求数。" },
  AI_PUBLIC_GLOBAL_PER_MINUTE: { label: "AI 全局限额", hint: "所有公开 AI 请求合计每分钟上限。" },
  AI_PUBLIC_MAX_CONCURRENCY: { label: "AI 全局并发", hint: "同时执行的公开 AI 请求上限，超出返回 429。" },
  BLOG_URL: { label: "博客地址", hint: "导航栏和关于页的博客入口。" },
  SITE_NAME: { label: "站点名称", hint: "服务端站点配置名称。" },
  SITE_DESCRIPTION: { label: "站点描述", hint: "首页主描述文案。" },
  ADMIN_PASSWORD: { label: "后台密码", hint: "至少 12 位；默认值会禁用后台登录。" },
  JWT_SECRET: { label: "JWT 密钥", hint: "至少 32 位；默认值会禁用后台登录。" },
  ADMIN_COOKIE_SECURE: { label: "管理 Cookie Secure", hint: "HTTPS 部署设为 true；留空按 WEB_ORIGIN 自动判断。" },
  REDIS_ADDR: { label: "Redis 地址", hint: "例如 localhost:6379 或 redis:6379。" },
  REDIS_PASSWORD: { label: "Redis 密码", hint: "没有密码时可以留空。" },
  REDIS_DB: { label: "Redis DB", hint: "缓存使用的数据库编号。" },
  SERVER_PORT: { label: "后端端口", hint: "Go API 服务监听端口。" },
  WEB_ORIGIN: { label: "Web 来源", hint: "允许跨域访问 API 的前端地址。" },
  TRUSTED_PROXIES: { label: "可信代理", hint: "逗号分隔的 IP/CIDR；用于正确识别客户端 IP。" },
  NEXT_PUBLIC_API_BASE_URL: { label: "前端 API 地址", hint: "浏览器请求后端 API 的地址。" },
  NEXT_PUBLIC_BLOG_URL: { label: "前端博客地址", hint: "浏览器中实际跳转的博客入口。" },
  NEXT_PUBLIC_GITHUB_URL: { label: "前端 GitHub 地址", hint: "导航栏 GitHub 图标跳转地址。" },
  NEXT_PUBLIC_SITE_NAME: { label: "前端站点名称", hint: "浏览器中显示的站点名称。" },
  NEXT_PUBLIC_SITE_DESCRIPTION: { label: "前端站点描述", hint: "浏览器中显示的首页描述。" },
};
