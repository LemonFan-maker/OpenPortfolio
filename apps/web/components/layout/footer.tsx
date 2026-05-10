import { siteName } from "@/lib/site-config";

export function Footer() {
  return (
    <footer className="border-t border-[var(--line)] px-6 py-8 text-sm text-[var(--muted)]">
      <div className="container flex flex-col gap-2 md:flex-row md:items-center md:justify-between">
        <span>{siteName}</span>
        <span>GitHub projects, language stats, and AI summaries.</span>
      </div>
    </footer>
  );
}
