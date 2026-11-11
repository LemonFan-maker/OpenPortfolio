import { afterEach, describe, expect, it, vi } from "vitest";

import { getCommitActivity, getLanguageStats, getProject, getProjects } from "./api";

describe("API fallbacks", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("returns empty data instead of baked sample content", async () => {
    vi.stubGlobal("fetch", vi.fn(async () => Promise.reject(new Error("offline"))));

    await expect(getProjects()).resolves.toEqual([]);
    await expect(getLanguageStats()).resolves.toEqual({ totalBytes: 0, languages: [] });
    await expect(getCommitActivity()).resolves.toEqual({
      totalContributions: 0,
      maxContributions: 0,
      weeks: [],
      months: [],
    });
    await expect(getProject("demo")).resolves.toBeNull();
  });
});
