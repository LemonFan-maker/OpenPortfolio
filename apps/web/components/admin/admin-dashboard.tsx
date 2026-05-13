"use client";

import {
  clearAdminAIProjectSummaries,
  clearAdminAISummaries,
  clearAdminAISiteSummary,
  clearAdminRepoBrowser,
  clearAdminRepoDetails,
  generateAdminAISummary,
  type EnvEntry,
  getAdminCacheRecords,
  getAdminFeatured,
  getAdminEnv,
  getAdminHiddenProjects,
  loginAdmin,
  refreshAdminActivity,
  refreshAdminAll,
  refreshAdminLanguages,
  refreshAdminRepos,
  saveAdminEnv,
  saveAdminFeatured,
  saveAdminHiddenProjects,
} from "@/lib/admin-api";
import type { CacheOperationRecord, CommitActivity, LanguageStats, ProjectCard, ProjectSummaryDebug } from "@/lib/types";
import {
  Activity,
  Bot,
  CheckCircle2,
  ChevronDown,
  Database,
  EyeOff,
  Github,
  Globe2,
  KeyRound,
  Loader2,
  Lock,
  LogOut,
  Plus,
  RefreshCcw,
  Save,
  Server,
  ShieldCheck,
  Sparkles,
  Star,
  Trash2,
  Wand2,
} from "lucide-react";
import { FormEvent, useEffect, useMemo, useState } from "react";

const tokenStorageKey = "portfolio-hub-admin-token";

type Operation = {
  description: string;
  group: string;
  icon: typeof RefreshCcw;
  id: string;
  run: (token: string) => Promise<CacheOperationRecord>;
  title: string;
};

type AdminDashboardProps = {
  activity: CommitActivity;
  languages: LanguageStats;
  projects: ProjectCard[];
};

type EnvGroupDefinition = {
  description: string;
  icon: typeof Database;
  id: string;
  keys: string[];
  title: string;
};

const envGroupDefinitions: EnvGroupDefinition[] = [
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
    keys: ["AI_API_KEY", "AI_BASE_URL", "AI_MODEL"],
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
    keys: ["ADMIN_PASSWORD", "JWT_SECRET"],
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
    keys: ["SERVER_PORT", "WEB_ORIGIN"],
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

const envFieldLabels: Record<string, { label: string; hint: string }> = {
  GITHUB_USERNAME: { label: "GitHub 用户名", hint: "用于拉取公开仓库、语言统计和 README。" },
  GITHUB_TOKEN: { label: "GitHub Token", hint: "用于贡献记录和提高 API 限额。" },
  FEATURED_REPOS: { label: "精选项目", hint: "英文逗号分隔，首页会优先按这个顺序展示。" },
  HIDDEN_REPOS: { label: "隐藏项目", hint: "英文逗号分隔，不在前台展示这些仓库。" },
  AI_API_KEY: { label: "AI API Key", hint: "仅后端使用，不会暴露给前端。" },
  AI_BASE_URL: { label: "AI Base URL", hint: "OpenAI 兼容接口地址。" },
  AI_MODEL: { label: "AI 模型", hint: "生成项目简介时使用的模型名称。" },
  BLOG_URL: { label: "博客地址", hint: "导航栏和关于页的博客入口。" },
  SITE_NAME: { label: "站点名称", hint: "服务端站点配置名称。" },
  SITE_DESCRIPTION: { label: "站点描述", hint: "首页主描述文案。" },
  ADMIN_PASSWORD: { label: "后台密码", hint: "管理员登录口令。" },
  JWT_SECRET: { label: "JWT 密钥", hint: "用于签发后台登录 Token。" },
  REDIS_ADDR: { label: "Redis 地址", hint: "例如 localhost:6379 或 redis:6379。" },
  REDIS_PASSWORD: { label: "Redis 密码", hint: "没有密码时可以留空。" },
  REDIS_DB: { label: "Redis DB", hint: "缓存使用的数据库编号。" },
  SERVER_PORT: { label: "后端端口", hint: "Go API 服务监听端口。" },
  WEB_ORIGIN: { label: "Web 来源", hint: "允许跨域访问 API 的前端地址。" },
  NEXT_PUBLIC_API_BASE_URL: { label: "前端 API 地址", hint: "浏览器请求后端 API 的地址。" },
  NEXT_PUBLIC_BLOG_URL: { label: "前端博客地址", hint: "浏览器中实际跳转的博客入口。" },
  NEXT_PUBLIC_GITHUB_URL: { label: "前端 GitHub 地址", hint: "导航栏 GitHub 图标跳转地址。" },
  NEXT_PUBLIC_SITE_NAME: { label: "前端站点名称", hint: "浏览器中显示的站点名称。" },
  NEXT_PUBLIC_SITE_DESCRIPTION: { label: "前端站点描述", hint: "浏览器中显示的首页描述。" },
};

export function AdminDashboard({ activity, languages, projects }: AdminDashboardProps) {
  const [password, setPassword] = useState("");
  const [token, setToken] = useState("");
  const [loginState, setLoginState] = useState<"idle" | "loading">("idle");
  const [loginError, setLoginError] = useState("");
  const [activeOperation, setActiveOperation] = useState("");
  const [notice, setNotice] = useState("");
  const [error, setError] = useState("");
  const [featuredText, setFeaturedText] = useState("");
  const [hiddenText, setHiddenText] = useState("");
  const [envPath, setEnvPath] = useState("");
  const [envEntries, setEnvEntries] = useState<EnvEntry[]>([]);
  const [cacheRecords, setCacheRecords] = useState<CacheOperationRecord[]>([]);
  const [loadingConfig, setLoadingConfig] = useState(false);
  const [savingConfig, setSavingConfig] = useState("");
  const [savingEnv, setSavingEnv] = useState(false);
  const [aiRepoName, setAiRepoName] = useState(projects[0]?.name ?? "");
  const [aiForceRefresh, setAiForceRefresh] = useState(true);
  const [aiLoading, setAiLoading] = useState(false);
  const [aiDebug, setAiDebug] = useState<ProjectSummaryDebug | null>(null);

  const stats = useMemo(
    () => [
      {
        icon: Database,
        label: "项目总数",
        value: projects.length.toString(),
      },
      {
        icon: Star,
        label: "累计 Star",
        value: projects.reduce((total, project) => total + project.stars, 0).toString(),
      },
      {
        icon: Activity,
        label: "年度贡献",
        value: activity.totalContributions.toString(),
      },
      {
        icon: Sparkles,
        label: "语言种类",
        value: languages.languages.length.toString(),
      },
    ],
    [activity.totalContributions, languages.languages.length, projects],
  );

  const operations: Operation[] = useMemo(
    () => [
      {
        description: "重新拉取 GitHub 仓库列表和项目基础信息。",
        group: "GitHub 数据",
        icon: Database,
        id: "repos",
        run: refreshAdminRepos,
        title: "项目列表",
      },
      {
        description: "重新统计公开仓库语言占比数据。",
        group: "GitHub 数据",
        icon: Activity,
        id: "languages",
        run: refreshAdminLanguages,
        title: "语言统计",
      },
      {
        description: "重新拉取年度贡献热力图数据。",
        group: "GitHub 数据",
        icon: Github,
        id: "activity",
        run: refreshAdminActivity,
        title: "贡献活动",
      },
      {
        description: "清理仓库详情和 README 缓存，下一次访问时重新拉取。",
        group: "仓库浏览",
        icon: Server,
        id: "repo-details",
        run: clearAdminRepoDetails,
        title: "仓库详情",
      },
      {
        description: "清理目录树、文件摘要和文件正文缓存。",
        group: "仓库浏览",
        icon: Database,
        id: "repo-browser",
        run: clearAdminRepoBrowser,
        title: "目录与文件",
      },
      {
        description: "清空 AI 项目简介缓存，下一次访问时重新生成。",
        group: "AI 生成",
        icon: Sparkles,
        id: "ai-projects",
        run: clearAdminAIProjectSummaries,
        title: "项目简介",
      },
      {
        description: "清空首页站点简介缓存，下一次访问时重新汇总。",
        group: "AI 生成",
        icon: Bot,
        id: "ai-site",
        run: clearAdminAISiteSummary,
        title: "站点简介",
      },
      {
        description: "一次性清空项目简介和站点简介缓存。",
        group: "AI 生成",
        icon: Sparkles,
        id: "ai-all",
        run: clearAdminAISummaries,
        title: "全部 AI",
      },
      {
        description: "清理派生缓存，并按顺序刷新项目、语言和贡献活动。",
        group: "全局操作",
        icon: RefreshCcw,
        id: "all",
        run: refreshAdminAll,
        title: "完整刷新",
      },
    ],
    [],
  );

  const operationGroups = useMemo(() => {
    const groups: { name: string; operations: Operation[] }[] = [];
    for (const operation of operations) {
      let group = groups.find((item) => item.name === operation.group);
      if (!group) {
        group = { name: operation.group, operations: [] };
        groups.push(group);
      }
      group.operations.push(operation);
    }
    return groups;
  }, [operations]);

  const topProjects = useMemo(
    () => [...projects].sort((a, b) => b.stars - a.stars).slice(0, 5),
    [projects],
  );

  const selectedAiProject = useMemo(
    () => projects.find((project) => project.name === aiRepoName) ?? null,
    [aiRepoName, projects],
  );

  useEffect(() => {
    const stored = window.localStorage.getItem(tokenStorageKey);
    if (stored) {
      setToken(stored);
    }
  }, []);

  useEffect(() => {
    if (!aiRepoName && projects.length > 0) {
      setAiRepoName(projects[0].name);
    }
  }, [aiRepoName, projects]);

  useEffect(() => {
    if (!token) {
      return;
    }

    let ignore = false;
    async function loadConfig() {
      setLoadingConfig(true);
      try {
        const [featured, hidden, envFile, records] = await Promise.all([
          getAdminFeatured(token),
          getAdminHiddenProjects(token),
          getAdminEnv(token),
          getAdminCacheRecords(token),
        ]);
        if (!ignore) {
          setFeaturedText(featured.join("\n"));
          setHiddenText(hidden.join("\n"));
          setEnvPath(envFile.path);
          setEnvEntries(envFile.entries);
          setCacheRecords(records);
        }
      } catch (nextError) {
        if (!ignore) {
          setError(nextError instanceof Error ? nextError.message : "读取管理配置失败");
        }
      } finally {
        if (!ignore) {
          setLoadingConfig(false);
        }
      }
    }

    void loadConfig();
    return () => {
      ignore = true;
    };
  }, [token]);

  async function loadCacheRecords(nextToken = token) {
    if (!nextToken) {
      return;
    }
    const records = await getAdminCacheRecords(nextToken);
    setCacheRecords(records);
  }

  async function handleLogin(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setLoginError("");
    setNotice("");
    setLoginState("loading");

    try {
      const nextToken = await loginAdmin(password);
      window.localStorage.setItem(tokenStorageKey, nextToken);
      setToken(nextToken);
      setPassword("");
      setNotice("已登录管理后台");
    } catch (nextError) {
      setLoginError(nextError instanceof Error ? nextError.message : "登录失败");
    } finally {
      setLoginState("idle");
    }
  }

  async function runOperation(operation: Operation) {
    setActiveOperation(operation.id);
    setError("");
    setNotice("");

    try {
      const record = await operation.run(token);
      setCacheRecords((current) => [record, ...current.filter((item) => item.id !== record.id)].slice(0, 30));
      setNotice(record.affectedKeys > 0 ? `${operation.title}已完成，影响 ${record.affectedKeys} 个缓存键` : `${operation.title}已完成`);
      void loadCacheRecords();
    } catch (nextError) {
      setError(nextError instanceof Error ? nextError.message : "操作失败");
      void loadCacheRecords();
    } finally {
      setActiveOperation("");
    }
  }

  async function saveList(kind: "featured" | "hidden") {
    const text = kind === "featured" ? featuredText : hiddenText;
    const names = text
      .split("\n")
      .map((item) => item.trim())
      .filter(Boolean);

    setSavingConfig(kind);
    setError("");
    setNotice("");

    try {
      if (kind === "featured") {
        await saveAdminFeatured(token, names);
      } else {
        await saveAdminHiddenProjects(token, names);
      }
      setNotice(kind === "featured" ? "精选项目已提交" : "隐藏项目已提交");
    } catch (nextError) {
      setError(nextError instanceof Error ? nextError.message : "保存失败");
    } finally {
      setSavingConfig("");
    }
  }

  async function saveEnv() {
    setSavingEnv(true);
    setError("");
    setNotice("");

    try {
      const envFile = await saveAdminEnv(token, envEntries);
      setEnvPath(envFile.path);
      setEnvEntries(envFile.entries);
      setNotice(".env 参数已保存，运行中的服务可能需要重启后完全生效");
    } catch (nextError) {
      setError(nextError instanceof Error ? nextError.message : "保存 .env 失败");
    } finally {
      setSavingEnv(false);
    }
  }

  function updateEnvEntry(index: number, field: keyof EnvEntry, value: string) {
    setEnvEntries((current) =>
      current.map((entry, entryIndex) => (entryIndex === index ? { ...entry, [field]: value } : entry)),
    );
  }

  function addEnvEntry() {
    setEnvEntries((current) => [...current, { key: "", value: "" }]);
  }

  function removeEnvEntry(index: number) {
    setEnvEntries((current) => current.filter((_, entryIndex) => entryIndex !== index));
  }

  function logout() {
    window.localStorage.removeItem(tokenStorageKey);
    setToken("");
    setNotice("");
    setError("");
    setFeaturedText("");
    setHiddenText("");
    setEnvPath("");
    setEnvEntries([]);
    setCacheRecords([]);
    setAiRepoName(projects[0]?.name ?? "");
    setAiForceRefresh(true);
    setAiLoading(false);
    setAiDebug(null);
  }

  async function generateAiSummary() {
    if (!aiRepoName) {
      setError("请先选择一个项目");
      return;
    }

    setAiLoading(true);
    setError("");
    setNotice("");
    setAiDebug(null);

    try {
      const result = await generateAdminAISummary(token, aiRepoName, aiForceRefresh);
      setAiDebug(result);
      setNotice(`已生成 ${result.repoName} 的 AI 简介`);
    } catch (nextError) {
      setError(nextError instanceof Error ? nextError.message : "生成 AI 简介失败");
    } finally {
      setAiLoading(false);
    }
  }

  if (!token) {
    return (
      <section className="section">
        <div className="container grid gap-8 lg:grid-cols-[1fr_420px]">
          <div className="self-center">
            <p className="eyebrow text-xs font-black uppercase">Admin Console</p>
            <h1 className="mt-4 max-w-2xl text-4xl font-black leading-tight md:text-5xl">
              后台管理界面
            </h1>
            <p className="mt-4 max-w-2xl leading-7 text-[var(--muted)]">
              管理 GitHub 项目缓存、语言统计、AI 简介缓存，以及精选和隐藏项目配置。
            </p>
            <div className="mt-8 grid gap-3 sm:grid-cols-2">
              {stats.map((item) => {
                const Icon = item.icon;
                return (
                  <div className="surface flex items-center gap-3 p-4" key={item.label}>
                    <Icon className="shrink-0 text-[var(--accent-warm)]" size={20} />
                    <div>
                      <div className="text-2xl font-black">{item.value}</div>
                      <div className="text-sm text-[var(--muted)]">{item.label}</div>
                    </div>
                  </div>
                );
              })}
            </div>
          </div>

          <form className="surface self-center p-6" onSubmit={handleLogin}>
            <div className="mb-6 flex items-start gap-3">
              <div className="flex h-11 w-11 items-center justify-center rounded-lg border border-[var(--line)] bg-[var(--background)]">
                <Lock size={20} />
              </div>
              <div>
                <h2 className="text-2xl font-black">管理员登录</h2>
                <p className="mt-1 text-sm text-[var(--muted)]">使用服务端 ADMIN_PASSWORD 登录。</p>
              </div>
            </div>
            <label className="block">
              <span className="text-sm font-bold">管理密码</span>
              <span className="mt-2 flex min-h-12 items-center gap-2 rounded-lg border border-[var(--line)] bg-[var(--background)] px-3">
                <KeyRound size={18} />
                <input
                  className="w-full bg-transparent outline-none"
                  onChange={(event) => setPassword(event.target.value)}
                  placeholder="输入管理密码"
                  type="password"
                  value={password}
                />
              </span>
            </label>
            {loginError ? (
              <div className="mt-4 rounded-lg border border-[color-mix(in_srgb,var(--accent-warm)_46%,var(--line))] p-3 text-sm text-[var(--accent-warm)]">
                {loginError}
              </div>
            ) : null}
            <button className="btn btn-primary mt-5 w-full" disabled={loginState === "loading"} type="submit">
              {loginState === "loading" ? <Loader2 className="animate-spin" size={18} /> : <ShieldCheck size={18} />}
              登录后台
            </button>
          </form>
        </div>
      </section>
    );
  }

  return (
    <section className="section">
      <div className="container space-y-6">
        <div className="flex flex-wrap items-end justify-between gap-4">
          <div>
            <p className="eyebrow text-xs font-black uppercase">Admin Console</p>
            <h1 className="mt-3 text-4xl font-black">后台管理界面</h1>
            <p className="mt-2 max-w-2xl text-[var(--muted)]">缓存刷新、内容配置和项目状态集中管理。</p>
          </div>
          <button className="btn btn-secondary" onClick={logout} type="button">
            <LogOut size={18} />
            退出登录
          </button>
        </div>

        {notice ? (
          <div className="flex items-center gap-2 rounded-lg border border-[color-mix(in_srgb,var(--accent-leaf)_46%,var(--line))] bg-[color-mix(in_srgb,var(--accent-leaf)_10%,var(--panel))] p-3 text-sm font-semibold">
            <CheckCircle2 size={18} />
            {notice}
          </div>
        ) : null}
        {error ? (
          <div className="rounded-lg border border-[color-mix(in_srgb,var(--accent-warm)_48%,var(--line))] bg-[color-mix(in_srgb,var(--accent-warm)_10%,var(--panel))] p-3 text-sm font-semibold text-[var(--accent-warm)]">
            {error}
          </div>
        ) : null}

        <div className="grid gap-4 md:grid-cols-4">
          {stats.map((item) => {
            const Icon = item.icon;
            return (
              <div className="surface p-4" key={item.label}>
                <Icon className="text-[var(--accent-warm)]" size={20} />
                <div className="mt-4 text-3xl font-black">{item.value}</div>
                <div className="mt-1 text-sm text-[var(--muted)]">{item.label}</div>
              </div>
            );
          })}
        </div>

        <section className="surface p-5">
          <div className="mb-5 flex flex-wrap items-start justify-between gap-3">
            <div>
              <h2 className="text-2xl font-black">AI 简介调试</h2>
              <p className="mt-1 text-sm text-[var(--muted)]">
                选择项目后直接生成，并在右侧查看后端返回的完整调试内容。
              </p>
            </div>
            <Sparkles className="text-[var(--accent-warm)]" size={20} />
          </div>

          <div className="grid gap-4 lg:grid-cols-[340px_1fr]">
            <div className="space-y-4">
              <label className="block">
                <span className="text-sm font-bold">项目</span>
                <span className="mt-2 flex min-h-12 items-center gap-2 rounded-lg border border-[var(--line)] bg-[var(--background)] px-3">
                  <select
                    className="w-full appearance-none bg-transparent outline-none"
                    onChange={(event) => setAiRepoName(event.target.value)}
                    value={aiRepoName}
                  >
                    <option value="">请选择项目</option>
                    {projects.map((project) => (
                      <option key={project.name} value={project.name}>
                        {project.name}
                      </option>
                    ))}
                  </select>
                  <ChevronDown className="shrink-0 text-[var(--muted)]" size={16} />
                </span>
              </label>

              <label className="flex items-center gap-3 rounded-lg border border-[var(--line)] bg-[var(--background)] px-3 py-3">
                <input
                  checked={aiForceRefresh}
                  onChange={(event) => setAiForceRefresh(event.target.checked)}
                  type="checkbox"
                />
                <span className="text-sm font-semibold">强制重新生成</span>
              </label>

              <button
                className="btn btn-primary w-full"
                disabled={aiLoading || !aiRepoName}
                onClick={() => void generateAiSummary()}
                type="button"
              >
                {aiLoading ? <Loader2 className="animate-spin" size={18} /> : <Wand2 size={18} />}
                生成 AI 简介
              </button>

              {selectedAiProject ? (
                <div className="rounded-lg border border-[var(--line)] bg-[color-mix(in_srgb,var(--panel)_66%,transparent)] p-4">
                  <div className="text-sm font-bold">{selectedAiProject.name}</div>
                  <div className="mt-2 text-sm leading-6 text-[var(--muted)]">
                    {selectedAiProject.description || "暂无项目描述"}
                  </div>
                </div>
              ) : null}
            </div>

            <div className="min-h-80 rounded-lg border border-[var(--line)] bg-[var(--background)] p-4">
              {aiDebug ? (
                <div className="space-y-4">
                  <div className="grid gap-3 md:grid-cols-3">
                    <InfoBlock label="来源" value={aiDebug.source} />
                    <InfoBlock label="缓存" value={aiDebug.cached ? "命中" : "未命中"} />
                    <InfoBlock label="轮次" value={aiDebug.turns.length.toString()} />
                  </div>

                  {aiDebug.error ? <InfoBlock label="错误" value={aiDebug.error} /> : null}

                  <div className="grid gap-3 md:grid-cols-2">
                    <TextBlock label="一句话简介" value={aiDebug.summary.shortSummary} />
                    <TextBlock label="详细简介" value={aiDebug.summary.longSummary} />
                  </div>

                  <div className="grid gap-3 md:grid-cols-2">
                    <TextBlock label="亮点" value={aiDebug.summary.highlights.join("\n")} />
                    <TextBlock label="技术标签" value={aiDebug.summary.techTags.join("\n")} />
                  </div>

                  <div>
                    <h3 className="text-sm font-black uppercase text-[var(--muted)]">完整返回</h3>
                    <pre className="mt-2 max-h-[36rem] overflow-auto rounded-lg border border-[var(--line)] bg-[var(--panel)] p-4 text-xs leading-6 text-[var(--foreground)]">
                      {JSON.stringify(aiDebug, null, 2)}
                    </pre>
                  </div>
                </div>
              ) : (
                <div className="flex h-full min-h-64 items-center justify-center text-sm text-[var(--muted)]">
                  生成后会在这里显示完整调试内容
                </div>
              )}
            </div>
          </div>
        </section>

        <section className="surface p-5">
          <div className="mb-5 flex items-start justify-between gap-3">
            <div>
              <h2 className="text-2xl font-black">缓存刷新</h2>
              <p className="mt-1 text-sm text-[var(--muted)]">按数据来源和派生内容分开处理，每次操作都会留下记录。</p>
            </div>
            <RefreshCcw size={20} />
          </div>
          <div className="grid gap-4 lg:grid-cols-4">
            {operationGroups.map((group) => (
              <div className="rounded-lg border border-[var(--line)] bg-[color-mix(in_srgb,var(--panel)_66%,transparent)] p-3" key={group.name}>
                <div className="mb-3 text-xs font-black uppercase text-[var(--muted)]">{group.name}</div>
                <div className="space-y-2">
                  {group.operations.map((operation) => {
                    const Icon = operation.icon;
                    const loading = activeOperation === operation.id;
                    return (
                      <button
                        className="flex min-h-12 w-full items-center gap-3 rounded-lg border border-[var(--line)] bg-[var(--background)] px-3 py-2 text-left transition hover:border-[color-mix(in_srgb,var(--accent-warm)_48%,var(--line))]"
                        disabled={Boolean(activeOperation)}
                        key={operation.id}
                        onClick={() => void runOperation(operation)}
                        title={operation.description}
                        type="button"
                      >
                        <Icon className="shrink-0 text-[var(--accent-warm)]" size={17} />
                        <span className="min-w-0 flex-1">
                          <span className="block truncate text-sm font-black">{operation.title}</span>
                          <span className="mt-0.5 block truncate text-xs text-[var(--muted)]">{operation.description}</span>
                        </span>
                        {loading ? <Loader2 className="shrink-0 animate-spin" size={16} /> : null}
                      </button>
                    );
                  })}
                </div>
              </div>
            ))}
          </div>
        </section>

        <section className="surface p-5">
          <div className="mb-5 flex items-start justify-between gap-3">
            <div>
              <h2 className="text-2xl font-black">刷新记录</h2>
              <p className="mt-1 text-sm text-[var(--muted)]">最近 30 次缓存操作。</p>
            </div>
            <Database size={20} />
          </div>
          <div className="grid gap-3 lg:grid-cols-2 xl:grid-cols-3">
            {cacheRecords.length > 0 ? (
              cacheRecords.map((record) => (
                <div
                  className="rounded-lg border border-[var(--line)] bg-[color-mix(in_srgb,var(--panel)_66%,transparent)] p-3"
                  key={record.id}
                >
                  <div className="flex flex-wrap items-center justify-between gap-2">
                    <div className="font-bold">{cacheScopeLabel(record.scope)}</div>
                    <span
                      className={
                        record.status === "success"
                          ? "rounded-full border border-[color-mix(in_srgb,var(--accent-leaf)_46%,var(--line))] px-2 py-1 text-xs font-bold text-[var(--accent-leaf)]"
                          : "rounded-full border border-[color-mix(in_srgb,var(--accent-warm)_46%,var(--line))] px-2 py-1 text-xs font-bold text-[var(--accent-warm)]"
                      }
                    >
                      {record.status === "success" ? "成功" : "失败"}
                    </span>
                  </div>
                  <div className="mt-2 text-sm leading-6 text-[var(--muted)]">{record.message}</div>
                  <div className="mt-3 grid gap-2 text-xs text-[var(--muted)] sm:grid-cols-3">
                    <span>{formatDateTime(record.finishedAt)}</span>
                    <span>{record.durationMs} ms</span>
                    <span>{record.affectedKeys} keys</span>
                  </div>
                  {record.patterns.length > 0 ? (
                    <div className="mt-3 flex flex-wrap gap-2">
                      {record.patterns.map((pattern) => (
                        <code
                          className="rounded-md border border-[var(--line)] bg-[var(--background)] px-2 py-1 text-xs text-[var(--muted)]"
                          key={pattern}
                        >
                          {pattern}
                        </code>
                      ))}
                    </div>
                  ) : null}
                </div>
              ))
            ) : (
              <div className="rounded-lg border border-[var(--line)] bg-[color-mix(in_srgb,var(--panel)_66%,transparent)] p-4 text-sm text-[var(--muted)]">
                暂无刷新记录
              </div>
            )}
          </div>
        </section>

        <div className="grid items-start gap-6 lg:grid-cols-[0.92fr_1.08fr]">
          <section className="surface p-5">
            <div className="mb-5 flex items-center justify-between gap-3">
              <div>
                <h2 className="text-2xl font-black">项目状态</h2>
                <p className="mt-1 text-sm text-[var(--muted)]">按 Star 排序的公开项目。</p>
              </div>
              <Star size={20} />
            </div>
            <div className="space-y-3">
              {topProjects.map((project) => (
                <a
                  className="block rounded-lg border border-[var(--line)] bg-[color-mix(in_srgb,var(--panel)_66%,transparent)] p-3 transition hover:border-[color-mix(in_srgb,var(--accent-warm)_48%,var(--line))]"
                  href={project.url}
                  key={project.name}
                  rel="noreferrer"
                  target="_blank"
                >
                  <div className="flex items-center justify-between gap-3">
                    <span className="truncate font-bold">{project.name}</span>
                    <span className="shrink-0 text-sm text-[var(--muted)]">{project.stars} stars</span>
                  </div>
                  <div className="mt-2 line-clamp-2 text-sm leading-6 text-[var(--muted)]">
                    {project.description || "暂无描述"}
                  </div>
                </a>
              ))}
            </div>
          </section>
        </div>

        <section className="grid gap-6 lg:grid-cols-2">
          <ConfigEditor
            busy={loadingConfig || savingConfig === "featured"}
            icon={Star}
            onChange={setFeaturedText}
            onSave={() => void saveList("featured")}
            title="精选项目"
            value={featuredText}
          />
          <ConfigEditor
            busy={loadingConfig || savingConfig === "hidden"}
            icon={EyeOff}
            onChange={setHiddenText}
            onSave={() => void saveList("hidden")}
            title="隐藏项目"
            value={hiddenText}
          />
        </section>

        <EnvEditor
          busy={loadingConfig || savingEnv}
          entries={envEntries}
          onAdd={addEnvEntry}
          onChange={updateEnvEntry}
          onRemove={removeEnvEntry}
          onSave={() => void saveEnv()}
          path={envPath}
        />
      </div>
    </section>
  );
}

function cacheScopeLabel(scope: string) {
  const labels: Record<string, string> = {
    "ai:all": "全部 AI 缓存",
    "ai:project-summary": "AI 项目简介",
    "ai:site-summary": "AI 站点简介",
    "cache:all": "完整缓存刷新",
    "github:activity": "GitHub 贡献活动",
    "github:languages": "GitHub 语言统计",
    "github:repo-browser": "仓库目录与文件",
    "github:repo-details": "仓库详情与 README",
    "github:repos": "GitHub 项目列表",
  };
  return labels[scope] ?? scope;
}

function formatDateTime(value: string) {
  if (!value) {
    return "";
  }
  return new Intl.DateTimeFormat("zh-CN", {
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    month: "2-digit",
  }).format(new Date(value));
}

function InfoBlock({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-lg border border-[var(--line)] bg-[color-mix(in_srgb,var(--panel)_66%,transparent)] p-3">
      <div className="text-xs font-black uppercase text-[var(--muted)]">{label}</div>
      <div className="mt-2 break-words text-sm font-semibold leading-6">{value}</div>
    </div>
  );
}

function TextBlock({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-lg border border-[var(--line)] bg-[color-mix(in_srgb,var(--panel)_66%,transparent)] p-3">
      <div className="text-xs font-black uppercase text-[var(--muted)]">{label}</div>
      <pre className="mt-2 max-h-44 overflow-auto whitespace-pre-wrap break-words text-sm leading-6 text-[var(--foreground)]">
        {value || "—"}
      </pre>
    </div>
  );
}

function ConfigEditor({
  busy,
  icon: Icon,
  onChange,
  onSave,
  title,
  value,
}: {
  busy: boolean;
  icon: typeof Star;
  onChange: (value: string) => void;
  onSave: () => void;
  title: string;
  value: string;
}) {
  return (
    <div className="surface p-5">
      <div className="mb-4 flex items-center justify-between gap-3">
        <div className="flex items-center gap-2">
          <Icon className="text-[var(--accent-warm)]" size={20} />
          <h2 className="text-2xl font-black">{title}</h2>
        </div>
        <button className="btn btn-secondary" disabled={busy} onClick={onSave} type="button">
          {busy ? <Loader2 className="animate-spin" size={18} /> : <Save size={18} />}
          保存
        </button>
      </div>
      <textarea
        className="min-h-44 w-full resize-y rounded-lg border border-[var(--line)] bg-[var(--background)] p-3 font-mono text-sm outline-none focus:border-[color-mix(in_srgb,var(--accent-warm)_60%,var(--line))]"
        onChange={(event) => onChange(event.target.value)}
        placeholder="每行一个仓库名称"
        value={value}
      />
    </div>
  );
}

function EnvEditor({
  busy,
  entries,
  onAdd,
  onChange,
  onRemove,
  onSave,
  path,
}: {
  busy: boolean;
  entries: EnvEntry[];
  onAdd: () => void;
  onChange: (index: number, field: keyof EnvEntry, value: string) => void;
  onRemove: (index: number) => void;
  onSave: () => void;
  path: string;
}) {
  const groupedEntries = useMemo(() => {
    const used = new Set<number>();
    const groups = envGroupDefinitions
      .map((group) => {
        const items = entries
          .map((entry, index) => ({ entry, index }))
          .filter((item) => group.keys.includes(item.entry.key));
        items.forEach((item) => used.add(item.index));
        return { ...group, items };
      })
      .filter((group) => group.items.length > 0);

    const otherItems = entries
      .map((entry, index) => ({ entry, index }))
      .filter((item) => !used.has(item.index));

    if (otherItems.length > 0) {
      groups.push({
        description: "未归类或新增的自定义环境参数。",
        icon: Sparkles,
        id: "other",
        items: otherItems,
        keys: [],
        title: "其他参数",
      });
    }

    return groups;
  }, [entries]);

  return (
    <section className="surface p-5">
      <div className="mb-5 flex flex-wrap items-start justify-between gap-3">
        <div>
          <h2 className="text-2xl font-black">环境参数</h2>
          <p className="mt-1 text-sm text-[var(--muted)]">
            {path ? path : "读取 .env 中"} 的全部键值。保存后按需重启 Web 或 Server。
          </p>
        </div>
        <div className="flex flex-wrap gap-2">
          <button className="btn btn-secondary" disabled={busy} onClick={onAdd} type="button">
            <Plus size={18} />
            新增
          </button>
          <button className="btn btn-primary" disabled={busy} onClick={onSave} type="button">
            {busy ? <Loader2 className="animate-spin" size={18} /> : <Save size={18} />}
            保存 .env
          </button>
        </div>
      </div>

      <div className="grid gap-5 xl:grid-cols-2">
        {groupedEntries.map((group) => {
          const Icon = group.icon;
          return (
            <section className="rounded-lg border border-[var(--line)] bg-[color-mix(in_srgb,var(--panel)_58%,transparent)] p-4" key={group.id}>
              <div className="mb-4 flex items-start gap-3">
                <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg border border-[var(--line)] bg-[var(--background)]">
                  <Icon className="text-[var(--accent-warm)]" size={18} />
                </div>
                <div className="min-w-0">
                  <div className="flex flex-wrap items-center gap-2">
                    <h3 className="text-lg font-black">{group.title}</h3>
                    <span className="rounded-md border border-[var(--line)] px-2 py-0.5 text-xs text-[var(--muted)]">
                      {group.items.length} 项
                    </span>
                  </div>
                  <p className="mt-1 text-sm leading-6 text-[var(--muted)]">{group.description}</p>
                </div>
              </div>

              <div className="space-y-3">
                {group.items.map(({ entry, index }) => {
                  const meta = envFieldLabels[entry.key] ?? {
                    label: entry.key || "自定义参数",
                    hint: "自定义环境变量。",
                  };
                  return (
                    <div className="rounded-lg border border-[var(--line)] bg-[var(--background)] p-3" key={`${entry.key}-${index}`}>
                      <div className="mb-3 flex items-start justify-between gap-3">
                        <div className="min-w-0">
                          <div className="font-bold leading-6">{meta.label}</div>
                          <div className="mt-1 text-xs leading-5 text-[var(--muted)]">{meta.hint}</div>
                        </div>
                        <button
                          aria-label={`删除 ${entry.key || "环境参数"}`}
                          className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg border border-[var(--line)] transition hover:border-[color-mix(in_srgb,var(--accent-warm)_54%,var(--line))]"
                          disabled={busy}
                          onClick={() => onRemove(index)}
                          title="删除"
                          type="button"
                        >
                          <Trash2 size={16} />
                        </button>
                      </div>

                      <div className="grid gap-2 md:grid-cols-[210px_1fr]">
                        <label className="block">
                          <span className="mb-1 block text-xs font-black uppercase text-[var(--muted)]">Key</span>
                          <input
                            className="min-h-11 w-full rounded-lg border border-[var(--line)] bg-[color-mix(in_srgb,var(--panel)_70%,transparent)] px-3 font-mono text-sm outline-none focus:border-[color-mix(in_srgb,var(--accent-warm)_60%,var(--line))]"
                            onChange={(event) => onChange(index, "key", event.target.value)}
                            placeholder="ENV_KEY"
                            value={entry.key}
                          />
                        </label>
                        <label className="block">
                          <span className="mb-1 block text-xs font-black uppercase text-[var(--muted)]">Value</span>
                          <input
                            className="min-h-11 w-full rounded-lg border border-[var(--line)] bg-[color-mix(in_srgb,var(--panel)_70%,transparent)] px-3 font-mono text-sm outline-none focus:border-[color-mix(in_srgb,var(--accent-warm)_60%,var(--line))]"
                            onChange={(event) => onChange(index, "value", event.target.value)}
                            placeholder="value"
                            type={isSensitiveEnvKey(entry.key) ? "password" : "text"}
                            value={entry.value}
                          />
                        </label>
                      </div>
                    </div>
                  );
                })}
              </div>
            </section>
          );
        })}
      </div>
    </section>
  );
}

function isSensitiveEnvKey(key: string) {
  return /(TOKEN|SECRET|PASSWORD|API_KEY|PRIVATE|CREDENTIAL)/i.test(key);
}
