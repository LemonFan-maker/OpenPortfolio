"use client";

import type { ProjectCard } from "@/lib/types";
import { Search } from "lucide-react";
import { useMemo, useState } from "react";
import { ProjectGrid } from "./project-grid";

export function ProjectBrowser({ projects }: { projects: ProjectCard[] }) {
  const [keyword, setKeyword] = useState("");
  const [language, setLanguage] = useState("all");
  const [sort, setSort] = useState("stars");

  const languages = useMemo(
    () =>
      Array.from(
        new Set(
          projects
            .flatMap((project) =>
              project.languages?.length ? project.languages.map((item) => item.name) : [project.language],
            )
            .filter(Boolean),
        ),
      ).sort(),
    [projects],
  );

  const filtered = useMemo(() => {
    const needle = keyword.trim().toLowerCase();
    return projects
      .filter((project) => {
        if (language === "all") {
          return true;
        }
        const projectLanguages = project.languages?.length
          ? project.languages.map((item) => item.name)
          : [project.language];
        return projectLanguages.some((item) => item === language);
      })
      .filter((project) => {
        if (!needle) {
          return true;
        }
        const projectLanguages = project.languages?.length
          ? project.languages.map((item) => item.name)
          : [project.language];
        return [project.name, project.description, ...projectLanguages, ...project.topics]
          .join(" ")
          .toLowerCase()
          .includes(needle);
      })
      .sort((a, b) => {
        if (sort === "name") {
          return a.name.localeCompare(b.name);
        }
        if (sort === "updated") {
          return projectTime(b.updatedAt) - projectTime(a.updatedAt) || a.name.localeCompare(b.name);
        }
        return b.stars - a.stars || projectTime(b.updatedAt) - projectTime(a.updatedAt);
      });
  }, [keyword, language, projects, sort]);

  return (
    <div className="space-y-6">
      <div className="surface grid gap-3 p-4 md:grid-cols-[1fr_180px_180px]">
        <label className="flex min-h-11 items-center gap-2 rounded-lg border border-[var(--line)] bg-[var(--background)] px-3">
          <Search size={18} />
          <input
            className="w-full bg-transparent outline-none"
            onChange={(event) => setKeyword(event.target.value)}
            placeholder="搜索项目"
            value={keyword}
          />
        </label>
        <select
          className="min-h-11 rounded-lg border border-[var(--line)] bg-[var(--background)] px-3"
          onChange={(event) => setLanguage(event.target.value)}
          value={language}
        >
          <option value="all">全部语言</option>
          {languages.map((item) => (
            <option key={item} value={item}>
              {item}
            </option>
          ))}
        </select>
        <select
          className="min-h-11 rounded-lg border border-[var(--line)] bg-[var(--background)] px-3"
          onChange={(event) => setSort(event.target.value)}
          value={sort}
        >
          <option value="stars">按 Star</option>
          <option value="updated">按更新时间</option>
          <option value="name">按名称</option>
        </select>
      </div>
      <ProjectGrid projects={filtered} />
    </div>
  );
}

function projectTime(value: string) {
  const timestamp = Date.parse(value);
  return Number.isNaN(timestamp) ? 0 : timestamp;
}
