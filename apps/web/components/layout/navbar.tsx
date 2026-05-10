"use client";

import { blogURL, githubURL, siteName } from "@/lib/site-config";
import { Github, Menu, X } from "lucide-react";
import Link from "next/link";
import { useState } from "react";
import { ThemeToggle } from "./theme-toggle";

export function Navbar() {
  const [open, setOpen] = useState(false);
  const close = () => setOpen(false);

  return (
    <header className="sticky top-0 z-40 border-b border-[var(--line)] bg-[color-mix(in_srgb,var(--background)_86%,transparent)] backdrop-blur">
      <div className="container flex min-h-16 items-center justify-between gap-4 px-6">
        <Link className="font-bold text-[var(--foreground)]" href="/" onClick={close}>
          {siteName}
        </Link>
        <nav className="hidden items-center gap-6 text-sm font-medium text-[var(--muted)] md:flex">
          <Link href="/">首页</Link>
          <Link href="/projects">项目</Link>
          <a href={blogURL}>博客</a>
          <Link href="/about">关于</Link>
        </nav>
        <div className="flex items-center gap-2">
          <a className="btn btn-secondary h-10 w-10 px-0" href={githubURL} rel="noreferrer" target="_blank" title="GitHub">
            <Github size={18} />
          </a>
          <ThemeToggle />
          <button
            aria-expanded={open}
            aria-label={open ? "关闭导航菜单" : "打开导航菜单"}
            className="btn btn-secondary h-10 w-10 px-0 md:hidden"
            onClick={() => setOpen((current) => !current)}
            type="button"
          >
            {open ? <X size={18} /> : <Menu size={18} />}
          </button>
        </div>
      </div>
      {open ? (
        <nav className="border-t border-[var(--line)] bg-[color-mix(in_srgb,var(--background)_94%,transparent)] px-6 py-3 text-sm font-semibold text-[var(--muted)] md:hidden">
          <div className="container grid gap-1">
            <Link className="rounded-md px-3 py-3 transition hover:bg-[color-mix(in_srgb,var(--panel)_78%,transparent)]" href="/" onClick={close}>
              首页
            </Link>
            <Link className="rounded-md px-3 py-3 transition hover:bg-[color-mix(in_srgb,var(--panel)_78%,transparent)]" href="/projects" onClick={close}>
              项目
            </Link>
            <a className="rounded-md px-3 py-3 transition hover:bg-[color-mix(in_srgb,var(--panel)_78%,transparent)]" href={blogURL} onClick={close}>
              博客
            </a>
            <Link className="rounded-md px-3 py-3 transition hover:bg-[color-mix(in_srgb,var(--panel)_78%,transparent)]" href="/about" onClick={close}>
              关于
            </Link>
          </div>
        </nav>
      ) : null}
    </header>
  );
}
