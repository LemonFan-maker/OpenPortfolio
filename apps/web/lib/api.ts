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
    [],
  );
}

export async function getLanguageStats(): Promise<LanguageStats> {
  return request<LanguageStats>(apiURL("/api/github/languages"), {
    totalBytes: 0,
    languages: [],
  });
}

export async function getCommitActivity(): Promise<CommitActivity> {
  return request<CommitActivity>(apiURL("/api/github/activity"), {
    totalContributions: 0,
    maxContributions: 0,
    weeks: [],
    months: [],
  });
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
  return request<RepoDetail | null>(
    apiURL(`/api/github/repos/${encodeURIComponent(name)}`),
    null,
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
