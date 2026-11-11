const defaultServerBaseURL = "http://localhost:7020";

export function normalizeApiBase(value: string | undefined): string {
  return (value ?? "").trim().replace(/\/+$/, "");
}

export function resolveApiBase(
  isServer: boolean,
  serverBase?: string,
  publicBase?: string,
): string {
  if (isServer) {
    return normalizeApiBase(serverBase) || defaultServerBaseURL;
  }
  return normalizeApiBase(publicBase);
}

export function joinApiURL(base: string, path: string): string {
  const normalizedPath = path.startsWith("/") ? path : `/${path}`;
  return `${normalizeApiBase(base)}${normalizedPath}`;
}

export function apiBaseURL(): string {
  if (typeof window === "undefined") {
    return resolveApiBase(true, process.env.API_BASE_URL);
  }
  return resolveApiBase(false, undefined, process.env.NEXT_PUBLIC_API_BASE_URL);
}

export function apiURL(path: string): string {
  return joinApiURL(apiBaseURL(), path);
}
