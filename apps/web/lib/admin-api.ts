import type { ApiResponse, CacheOperationRecord, ProjectSummaryDebug } from "./types";
import { apiURL } from "./api-base";

export type AdminActionResult = CacheOperationRecord;

export type EnvEntry = {
  key: string;
  value: string;
};

export type EnvFile = {
  path: string;
  entries: EnvEntry[];
};

export type AdminSession = {
  authenticated: boolean;
};

const csrfCookieName = "portfolio_admin_csrf";

export async function loginAdmin(password: string): Promise<void> {
  await adminFetch<AdminSession>("/api/admin/login", {
    body: JSON.stringify({ password }),
    method: "POST",
  });
}

export function getAdminSession(): Promise<AdminSession> {
  return adminFetch<AdminSession>("/api/admin/session");
}

export function logoutAdmin(): Promise<AdminSession> {
  return adminFetch<AdminSession>("/api/admin/logout", { method: "POST" });
}

export function refreshAdminRepos() {
  return adminFetch<AdminActionResult>("/api/admin/cache/refresh/repos", {
    method: "POST",
  });
}

export function refreshAdminLanguages() {
  return adminFetch<AdminActionResult>("/api/admin/cache/refresh/languages", {
    method: "POST",
  });
}

export function refreshAdminActivity() {
  return adminFetch<AdminActionResult>("/api/admin/cache/refresh/activity", {
    method: "POST",
  });
}

export function refreshAdminAll() {
  return adminFetch<AdminActionResult>("/api/admin/cache/refresh/all", {
    method: "POST",
  });
}

export function clearAdminRepoDetails() {
  return adminFetch<AdminActionResult>("/api/admin/cache/clear/repo-details", {
    method: "POST",
  });
}

export function clearAdminRepoBrowser() {
  return adminFetch<AdminActionResult>("/api/admin/cache/clear/repo-browser", {
    method: "POST",
  });
}

export function clearAdminAIProjectSummaries() {
  return adminFetch<AdminActionResult>("/api/admin/cache/clear/ai-projects", {
    method: "POST",
  });
}

export function clearAdminAISiteSummary() {
  return adminFetch<AdminActionResult>("/api/admin/cache/clear/ai-site", {
    method: "POST",
  });
}

export function clearAdminAISummaries() {
  return adminFetch<AdminActionResult>("/api/admin/cache/clear/ai", {
    method: "POST",
  });
}

export function getAdminCacheRecords() {
  return adminFetch<CacheOperationRecord[]>("/api/admin/cache/records");
}

export function generateAdminAISummary(repoName: string, forceRefresh: boolean) {
  return adminFetch<ProjectSummaryDebug>("/api/admin/ai/project-summary", {
    body: JSON.stringify({ repoName, forceRefresh }),
    method: "POST",
  });
}

export function getAdminEnv() {
  return adminFetch<EnvFile>("/api/admin/env");
}

export function saveAdminEnv(entries: EnvEntry[]) {
  return adminFetch<EnvFile>("/api/admin/env", {
    body: JSON.stringify({ entries }),
    method: "PUT",
  });
}

export function getAdminFeatured() {
  return adminFetch<string[]>("/api/admin/featured");
}

export function saveAdminFeatured(projects: string[]) {
  return adminFetch<AdminActionResult>("/api/admin/featured", {
    body: JSON.stringify({ projects }),
    method: "POST",
  });
}

export function getAdminHiddenProjects() {
  return adminFetch<string[]>("/api/admin/hidden-projects");
}

export function saveAdminHiddenProjects(projects: string[]) {
  return adminFetch<AdminActionResult>("/api/admin/hidden-projects", {
    body: JSON.stringify({ projects }),
    method: "POST",
  });
}

async function adminFetch<T>(path: string, init: RequestInit = {}): Promise<T> {
  const { headers, ...rest } = init;
  const method = (rest.method ?? "GET").toUpperCase();
  const csrf = readCSRFCookie();
  const needsCSRF = method === "POST" || method === "PUT" || method === "PATCH" || method === "DELETE";
  const res = await fetch(apiURL(path), {
    ...rest,
    cache: "no-store",
    credentials: "include",
    headers: {
      "Content-Type": "application/json",
      ...(needsCSRF && csrf ? { "X-CSRF-Token": csrf } : {}),
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

function readCSRFCookie(): string {
  if (typeof document === "undefined") {
    return "";
  }
  const prefix = `${csrfCookieName}=`;
  for (const part of document.cookie.split(";")) {
    const trimmed = part.trim();
    if (trimmed.startsWith(prefix)) {
      return decodeURIComponent(trimmed.slice(prefix.length));
    }
  }
  return "";
}
