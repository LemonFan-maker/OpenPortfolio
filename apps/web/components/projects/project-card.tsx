"use client";

import type { ProjectCard as ProjectCardType } from "@/lib/types";
import { motion, useReducedMotion } from "framer-motion";
import { ExternalLink, GitFork, Star } from "lucide-react";
import Link from "next/link";

export function ProjectCard({ project }: { project: ProjectCardType }) {
  const reduce = useReducedMotion();
  const languages = project.languages?.length
    ? project.languages
    : project.language
      ? [{ name: project.language, bytes: 0, percent: 0 }]
      : [];

  return (
    <motion.article
      className="surface mb-5 inline-block w-full break-inside-avoid p-5 transition duration-150"
      initial={reduce ? false : { opacity: 0, y: 18 }}
      whileInView={reduce ? undefined : { opacity: 1, y: 0 }}
      viewport={{ once: true, amount: 0.25 }}
      whileHover={reduce ? undefined : { y: -4 }}
      transition={{ duration: 0.55, ease: [0.22, 1, 0.36, 1] }}
    >
      <div className="flex items-start justify-between gap-4">
        <div className="min-w-0 flex-1">
          <Link
            className="block break-words text-lg font-bold leading-snug hover:text-[var(--accent)] sm:text-xl"
            href={`/projects/${project.name}`}
            title={project.name}
          >
            {project.name}
          </Link>
          {project.description ? (
            <p className="mt-3 text-sm leading-6 text-[var(--muted)]">{project.description}</p>
          ) : null}
        </div>
        <a className="mt-1 shrink-0 rounded-full border border-[var(--line)] p-2" href={project.url} title="Open on GitHub">
          <ExternalLink size={16} />
        </a>
      </div>
      {languages.length > 0 ? (
        <div className="mt-4 flex flex-wrap gap-2">
          {languages.map((language, index) => (
            <span
              className={
                index === 0
                  ? "rounded-md border border-[color-mix(in_srgb,var(--accent-warm)_34%,var(--line))] bg-[color-mix(in_srgb,var(--accent-warm)_18%,var(--panel))] px-2 py-1 text-xs font-semibold text-[var(--foreground)]"
                  : "rounded-md border border-[var(--line)] bg-[color-mix(in_srgb,var(--panel)_60%,transparent)] px-2 py-1 text-xs text-[var(--muted)]"
              }
              key={language.name}
            >
              {language.name}
            </span>
          ))}
        </div>
      ) : null}
      {project.topics.length > 0 && (
        <>
          <div className="my-4 h-px w-full bg-[var(--line)]" />
          <div className="flex flex-wrap gap-2">
            {project.topics.slice(0, 3).map((topic) => (
              <span
                className="rounded-md border border-[var(--line)] bg-[color-mix(in_srgb,var(--panel)_60%,transparent)] px-2 py-1 text-xs text-[var(--muted)]"
                key={topic}
              >
                {topic}
              </span>
            ))}
          </div>
        </>
      )}
      <div className="mt-6 flex items-center gap-4 text-sm text-[var(--muted)]">
        <span className="inline-flex items-center gap-1">
          <Star size={15} /> {project.stars}
        </span>
        <span className="inline-flex items-center gap-1">
          <GitFork size={15} /> {project.forks}
        </span>
        <span className="ml-auto">{formatDate(project.updatedAt)}</span>
      </div>
    </motion.article>
  );
}

function formatDate(value: string) {
  if (!value) {
    return "";
  }
  return new Intl.DateTimeFormat("zh-CN", { month: "short", day: "numeric" }).format(new Date(value));
}
