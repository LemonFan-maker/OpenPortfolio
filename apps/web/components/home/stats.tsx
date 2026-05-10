"use client";

import type { LanguageStats, ProjectCard } from "@/lib/types";
import { motion, useReducedMotion } from "framer-motion";

export function Stats({ projects, languages }: { projects: ProjectCard[]; languages: LanguageStats }) {
  const stars = projects.reduce((total, project) => total + project.stars, 0);
  const forks = projects.reduce((total, project) => total + project.forks, 0);
  const items = [
    { label: "开源项目", value: projects.length.toString() },
    { label: "Star 总数", value: stars.toString() },
    { label: "Fork 总数", value: forks.toString() },
    { label: "常用语言", value: languages.languages.length.toString() },
  ];

  return (
    <section className="section py-8">
      <div className="container grid gap-4 md:grid-cols-4">
        {items.map((item, index) => (
          <Stat key={item.label} delay={index * 0.08} label={item.label} value={item.value} />
        ))}
      </div>
    </section>
  );
}

function Stat({ delay, label, value }: { delay: number; label: string; value: string }) {
  const reduce = useReducedMotion();

  return (
    <motion.div
      className="surface p-5"
      initial={reduce ? false : { opacity: 0, y: 16, scale: 0.96 }}
      whileInView={reduce ? undefined : { opacity: 1, y: 0, scale: 1 }}
      viewport={{ once: true, amount: 0.4 }}
      whileHover={reduce ? undefined : { y: -2 }}
      transition={{ duration: 0.55, delay, ease: [0.22, 1, 0.36, 1] }}
    >
      <div className="text-3xl font-black">{value}</div>
      <div className="mt-1 text-sm text-[var(--muted)]">{label}</div>
    </motion.div>
  );
}
