import { afterEach, describe, expect, it, vi } from "vitest";

// These tests must re-evaluate the module after stubbing NEXT_PUBLIC_* env vars,
// so a static import (cached at module load) cannot be used here.

afterEach(() => {
  vi.unstubAllEnvs();
  vi.resetModules();
});

describe("site config", () => {
  it("falls back when public values are unset or blank", async () => {
    vi.stubEnv("NEXT_PUBLIC_SITE_NAME", "");
    vi.stubEnv("NEXT_PUBLIC_SITE_DESCRIPTION", "   ");
    const config = await import("./site-config");

    expect(config.siteName).toBe("OrionisLi.com");
    expect(config.siteDescription).toBe("代码如诗，构建无限可能");
  });

  it("falls back to server site values when public values are missing", async () => {
    vi.stubEnv("NEXT_PUBLIC_SITE_NAME", "");
    vi.stubEnv("NEXT_PUBLIC_SITE_DESCRIPTION", "");
    vi.stubEnv("SITE_NAME", " Server Portfolio ");
    vi.stubEnv("SITE_DESCRIPTION", " Server description ");
    const config = await import("./site-config");

    expect(config.siteName).toBe("Server Portfolio");
    expect(config.siteDescription).toBe("Server description");
  });

  it("trims and uses configured public values", async () => {
    vi.stubEnv("NEXT_PUBLIC_SITE_NAME", " My Portfolio ");
    vi.stubEnv("NEXT_PUBLIC_SITE_DESCRIPTION", " A personal site ");
    const config = await import("./site-config");

    expect(config.siteName).toBe("My Portfolio");
    expect(config.siteDescription).toBe("A personal site");
  });
});
