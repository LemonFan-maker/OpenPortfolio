import type { ApiResponse, CacheOperationRecord, ProjectSummaryDebug } from "./types";

const baseURL = process.env.NEXT_PUBLIC_API_BASE_URL ?? "http://localhost:8080";

export type AdminActionResult = CacheOperationRecord;

export type EnvEntry = {
  key: string;
  value: string;
};

export type EnvFile = {
  path: string;
  entries: EnvEntry[];
};

export async function loginAdmin(password: string): Promise<string> {
  const body = await adminFetch<{ token: string }>("/api/admin/login", {
    body: JSON.stringify({ password }),
    method: "POST",
  });
  return body.token;
}

export function refreshAdminRepos(token: string) {
  return adminFetch<AdminActionResult>("/api/admin/cache/refresh/repos", {
    method: "POST",
    token,
  });
}

export function refreshAdminLanguages(token: string) {
  return adminFetch<AdminActionResult>("/api/admin/cache/refresh/languages", {
    method: "POST",
    token,
  });
}

export function refreshAdminActivity(token: string) {
  return adminFetch<AdminActionResult>("/api/admin/cache/refresh/activity", {
    method: "POST",
    token,
  });
}

export function refreshAdminAll(token: string) {
  return adminFetch<AdminActionResult>("/api/admin/cache/refresh/all", {
    method: "POST",
    token,
  });
}

export function clearAdminRepoDetails(token: string) {
  return adminFetch<AdminActionResult>("/api/admin/cache/clear/repo-details", {
    method: "POST",
    token,
  });
}

export function clearAdminRepoBrowser(token: string) {
  return adminFetch<AdminActionResult>("/api/admin/cache/clear/repo-browser", {
    method: "POST",
    token,
  });
}

export function clearAdminAIProjectSummaries(token: string) {
  return adminFetch<AdminActionResult>("/api/admin/cache/clear/ai-projects", {
    method: "POST",
    token,
  });
}

export function clearAdminAISiteSummary(token: string) {
  return adminFetch<AdminActionResult>("/api/admin/cache/clear/ai-site", {
    method: "POST",
    token,
  });
}

export function clearAdminAISummaries(token: string) {
  return adminFetch<AdminActionResult>("/api/admin/cache/clear/ai", {
    method: "POST",
    token,
  });
}

export function getAdminCacheRecords(token: string) {
  return adminFetch<CacheOperationRecord[]>("/api/admin/cache/records", { token });
}

export function generateAdminAISummary(token: string, repoName: string, forceRefresh: boolean) {
  return adminFetch<ProjectSummaryDebug>("/api/admin/ai/project-summary", {
    body: JSON.stringify({ repoName, forceRefresh }),
    method: "POST",
    token,
  });
}

export function getAdminEnv(token: string) {
  return adminFetch<EnvFile>("/api/admin/env", { token });
}

export function saveAdminEnv(token: string, entries: EnvEntry[]) {
  return adminFetch<EnvFile>("/api/admin/env", {
    body: JSON.stringify({ entries }),
    method: "PUT",
    token,
  });
}

export function getAdminFeatured(token: string) {
  return adminFetch<string[]>("/api/admin/featured", { token });
}

export function saveAdminFeatured(token: string, projects: string[]) {
  return adminFetch<AdminActionResult>("/api/admin/featured", {
    body: JSON.stringify({ projects }),
    method: "POST",
    token,
  });
}

export function getAdminHiddenProjects(token: string) {
  return adminFetch<string[]>("/api/admin/hidden-projects", { token });
}

export function saveAdminHiddenProjects(token: string, projects: string[]) {
  return adminFetch<AdminActionResult>("/api/admin/hidden-projects", {
    body: JSON.stringify({ projects }),
    method: "POST",
    token,
  });
}

async function adminFetch<T>(
  path: string,
  init: RequestInit & { token?: string } = {},
): Promise<T> {
  const { token, headers, ...rest } = init;
  const res = await fetch(new URL(path, baseURL), {
    ...rest,
    cache: "no-store",
    headers: {
      "Content-Type": "application/json",
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
      ...headers,
    },
  });

  let body: ApiResponse<T>;
  try {
    body = (await res.json()) as ApiResponse<T>;
  } catch {
    throw new Error("管理接口返回了无法解析的数据");
  }

  if (!res.ok || !body.success || body.data === undefined) {
    throw new Error(body.error || `管理接口请求失败: ${res.status}`);
  }

  return body.data;
}
