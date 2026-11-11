import type { NextConfig } from "next";
import { existsSync, readFileSync } from "node:fs";
import { resolve } from "node:path";

loadRootEnv();
if (!process.env.NEXT_PUBLIC_SITE_NAME?.trim() && process.env.SITE_NAME?.trim()) {
  process.env.NEXT_PUBLIC_SITE_NAME = process.env.SITE_NAME;
}
if (!process.env.NEXT_PUBLIC_SITE_DESCRIPTION?.trim() && process.env.SITE_DESCRIPTION?.trim()) {
  process.env.NEXT_PUBLIC_SITE_DESCRIPTION = process.env.SITE_DESCRIPTION;
}

const apiBaseURL = (process.env.API_BASE_URL ?? "http://localhost:7020").replace(/\/+$/, "");

const nextConfig: NextConfig = {
  output: "standalone",
  async rewrites() {
    return [
      {
        source: "/api/:path*",
        destination: `${apiBaseURL}/api/:path*`,
      },
    ];
  },
};

export default nextConfig;

function loadRootEnv() {
  const envPath = resolve(process.cwd(), "../../.env");
  if (!existsSync(envPath)) {
    return;
  }

  for (const rawLine of readFileSync(envPath, "utf8").split(/\r?\n/)) {
    const line = rawLine.trim();
    if (!line || line.startsWith("#")) {
      continue;
    }

    const separator = line.indexOf("=");
    if (separator <= 0) {
      continue;
    }

    const key = line.slice(0, separator).trim();
    const value = line.slice(separator + 1).trim().replace(/^['"]|['"]$/g, "");
    if (key && process.env[key] === undefined) {
      process.env[key] = value;
    }
  }
}
