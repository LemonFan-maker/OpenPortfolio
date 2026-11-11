import { afterEach, describe, expect, it, vi } from "vitest";
import { apiURL, joinApiURL, normalizeApiBase, resolveApiBase } from "./api-base";

afterEach(() => {
  vi.unstubAllEnvs();
  vi.unstubAllGlobals();
});

describe("normalizeApiBase", () => {
  it("trims whitespace and trailing slashes", () => {
    expect(normalizeApiBase("  http://server:7020/  ")).toBe("http://server:7020");
  });

  it("keeps an empty value empty for same-origin URLs", () => {
    expect(normalizeApiBase(undefined)).toBe("");
    expect(normalizeApiBase("")).toBe("");
  });
});

describe("resolveApiBase", () => {
  it("defaults the server base to port 7020", () => {
    expect(resolveApiBase(true)).toBe("http://localhost:7020");
  });

  it("prefers the configured server base", () => {
    expect(resolveApiBase(true, "http://server:7020/")).toBe("http://server:7020");
  });

  it("uses an empty base in the browser for same-origin /api", () => {
    expect(resolveApiBase(false, "http://server:7020")).toBe("");
  });

  it("honors an explicit public base", () => {
    expect(resolveApiBase(false, undefined, "https://api.example.com/")).toBe(
      "https://api.example.com",
    );
  });
});

describe("joinApiURL", () => {
  it("keeps same-origin paths relative", () => {
    expect(joinApiURL("", "/api/health")).toBe("/api/health");
  });

  it("joins an absolute base and normalizes the leading slash", () => {
    expect(joinApiURL("http://localhost:7020/", "api/health")).toBe(
      "http://localhost:7020/api/health",
    );
  });
});

describe("apiURL", () => {
  it("returns a relative URL in jsdom by default", () => {
    expect(apiURL("/api/health")).toBe("/api/health");
  });

  it("uses NEXT_PUBLIC_API_BASE_URL in the browser", () => {
    vi.stubEnv("NEXT_PUBLIC_API_BASE_URL", "https://api.example.com");
    expect(apiURL("/api/health")).toBe("https://api.example.com/api/health");
  });

  it("uses API_BASE_URL when window is unavailable", () => {
    vi.stubGlobal("window", undefined);
    vi.stubEnv("API_BASE_URL", "http://server:7020");
    expect(apiURL("/api/health")).toBe("http://server:7020/api/health");
  });
});
