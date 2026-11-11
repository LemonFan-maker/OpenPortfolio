function envOrDefault(value: string | undefined, fallback: string): string {
  const trimmed = value?.trim();
  return trimmed ? trimmed : fallback;
}

export const siteName = envOrDefault(process.env.NEXT_PUBLIC_SITE_NAME, "OrionisLi.com");
export const siteDescription = envOrDefault(
  process.env.NEXT_PUBLIC_SITE_DESCRIPTION,
  "代码如诗，构建无限可能",
);
export const blogURL = envOrDefault(process.env.NEXT_PUBLIC_BLOG_URL, "#");
export const githubURL = envOrDefault(
  process.env.NEXT_PUBLIC_GITHUB_URL,
  "https://github.com/LemonFan-maker",
);
