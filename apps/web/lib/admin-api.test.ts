import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { getAdminSession, saveAdminFeatured } from "./admin-api";

function response<T>(data: T) {
  return {
    json: async () => ({ success: true, data }),
    ok: true,
    status: 200,
  } as Response;
}

describe("admin API", () => {
  beforeEach(() => {
    document.cookie = "portfolio_admin_csrf=csrf-token; path=/";
    vi.stubGlobal("fetch", vi.fn(async () => response({ authenticated: true })));
  });

  afterEach(() => {
    document.cookie = "portfolio_admin_csrf=; Max-Age=0; path=/";
    vi.unstubAllGlobals();
  });

  it("sends the CSRF cookie as a header on writes", async () => {
    await saveAdminFeatured(["demo"]);

    const fetchMock = vi.mocked(fetch);
    const init = fetchMock.mock.calls[0]?.[1] as RequestInit | undefined;
    const headers = new Headers(init?.headers);
    expect(headers.get("X-CSRF-Token")).toBe("csrf-token");
    expect(init?.credentials).toBe("include");
  });

  it("does not require a CSRF header for session reads", async () => {
    await getAdminSession();

    const fetchMock = vi.mocked(fetch);
    const init = fetchMock.mock.calls[0]?.[1] as RequestInit | undefined;
    const headers = new Headers(init?.headers);
    expect(headers.get("X-CSRF-Token")).toBeNull();
    expect(init?.credentials).toBe("include");
  });
});
