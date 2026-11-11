import type {
  ApiResponse,
  CommitActivity,
  CommitActivityDetail,
  LanguageStats,
  ProjectCard,
  ProjectSummaryStatus,
  SiteSummaryStatus,
  RepoDetail,
} from "./types";
import { apiURL } from "./api-base";

export async function getProjects(params?: {
  sort?: string;
  language?: string;
  keyword?: string;
}): Promise<ProjectCard[]> {
  const query = new URLSearchParams();
  Object.entries(params ?? {}).forEach(([key, value]) => {
    if (value) {
      query.set(key, value);
    }
  });

  const suffix = query.toString();
  return request<ProjectCard[]>(
    `${apiURL("/api/github/repos")}${suffix ? `?${suffix}` : ""}`,
    fallbackProjects,
  );
}

export async function getLanguageStats(): Promise<LanguageStats> {
  return request<LanguageStats>(apiURL("/api/github/languages"), {
    totalBytes: fallbackLanguages.reduce((total, item) => total + item.bytes, 0),
    languages: fallbackLanguages,
  });
}

export async function getCommitActivity(): Promise<CommitActivity> {
  return request<CommitActivity>(apiURL("/api/github/activity"), fallbackActivity);
}

export async function getCommitActivityDetail(date: string): Promise<CommitActivityDetail | null> {
  return request<CommitActivityDetail | null>(
    apiURL(`/api/github/activity/${encodeURIComponent(date)}`),
    null,
  );
}

export async function getProjectSummary(name: string): Promise<ProjectSummaryStatus | null> {
  return request<ProjectSummaryStatus | null>(
    apiURL("/api/ai/project-summary"),
    null,
    {
      body: JSON.stringify({ repoName: name, forceRefresh: false }),
      cache: "no-store",
      method: "POST",
    },
  );
}

export async function getSiteSummary(): Promise<SiteSummaryStatus | null> {
  return request<SiteSummaryStatus | null>(
    apiURL("/api/ai/site-summary"),
    null,
    {
      body: JSON.stringify({ forceRefresh: false }),
      cache: "no-store",
      method: "POST",
    },
  );
}

export async function getProject(name: string): Promise<RepoDetail | null> {
  const fallback = fallbackProjects.find((project) => project.name === name);
  const detailFallback = fallback
    ? {
        ...fallback,
        readme: "配置 GITHUB_USERNAME 后，这里会展示仓库 README 摘要。",
        languages: fallbackLanguages.slice(0, 3),
        openIssues: 0,
      }
    : null;
  return request<RepoDetail | null>(
    apiURL(`/api/github/repos/${encodeURIComponent(name)}`),
    detailFallback,
  );
}

async function request<T>(url: string, fallback: T, init: RequestInit = {}): Promise<T> {
  try {
    const { headers, ...rest } = init;
    const res = await fetch(url, {
      ...rest,
      headers: {
        "Content-Type": "application/json",
        ...(headers ?? {}),
      },
      next: { revalidate: 120 },
    });
    if (!res.ok) {
      return fallback;
    }
    const body = (await res.json()) as ApiResponse<T>;
    return body.success && body.data !== undefined ? body.data : fallback;
  } catch {
    return fallback;
  }
}

export const fallbackProjects: ProjectCard[] = [
  {
    name: "portfolio-hub",
    description: "个人 GitHub 开源项目展示站，聚合项目、语言统计和 AI 简介。",
    language: "TypeScript",
    languages: [
      { name: "TypeScript", bytes: 540000, percent: 62.1 },
      { name: "Go", bytes: 240000, percent: 27.6 },
      { name: "CSS", bytes: 90000, percent: 10.3 },
    ],
    stars: 128,
    forks: 12,
    topics: ["nextjs", "go", "redis"],
    url: "https://github.com/example/portfolio-hub",
    homepage: "",
    updatedAt: "2026-05-01T00:00:00Z",
  },
  {
    name: "api-craft",
    description: "Go 编写的轻量 API 服务，负责数据聚合和缓存管理。",
    language: "Go",
    languages: [
      { name: "Go", bytes: 360000, percent: 88.2 },
      { name: "Shell", bytes: 48000, percent: 11.8 },
    ],
    stars: 86,
    forks: 9,
    topics: ["gin", "github-api"],
    url: "https://github.com/example/api-craft",
    homepage: "",
    updatedAt: "2026-04-20T00:00:00Z",
  },
  {
    name: "notes-engine",
    description: "面向技术博客的内容整理和索引工具。",
    language: "Python",
    languages: [
      { name: "Python", bytes: 220000, percent: 79.7 },
      { name: "HTML", bytes: 38000, percent: 13.8 },
      { name: "CSS", bytes: 18000, percent: 6.5 },
    ],
    stars: 43,
    forks: 5,
    topics: ["blog", "automation"],
    url: "https://github.com/example/notes-engine",
    homepage: "",
    updatedAt: "2026-03-18T00:00:00Z",
  },
];

export const fallbackLanguages = [
  { name: "TypeScript", bytes: 540000, percent: 43.7 },
  { name: "Go", bytes: 360000, percent: 29.1 },
  { name: "Python", bytes: 220000, percent: 17.8 },
  { name: "CSS", bytes: 116000, percent: 9.4 },
];

const fallbackWeeks = Array.from({ length: 32 }, (_, weekIndex) => {
  const first = new Date(Date.UTC(2026, 0, 4 + weekIndex * 7));
  return {
    firstDay: first.toISOString().slice(0, 10),
    days: Array.from({ length: 7 }, (_, dayIndex) => {
      const date = new Date(first);
      date.setUTCDate(first.getUTCDate() + dayIndex);
      return {
        date: date.toISOString().slice(0, 10),
        count: 0,
        color: "#ebe7dc",
      };
    }),
  };
});

export const fallbackActivity: CommitActivity = {
  totalContributions: 0,
  maxContributions: 0,
  weeks: fallbackWeeks,
  months: [],
};
