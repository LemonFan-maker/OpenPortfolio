"use client";

import type { CommitActivity, ContributionWeek, LanguageStats, ProjectCard } from "@/lib/types";
import { blogURL, siteDescription, siteName } from "@/lib/site-config";
import { motion, useReducedMotion } from "framer-motion";
import { ArrowRight, BookOpen, Code2, Github, GitPullRequest, Star } from "lucide-react";
import Link from "next/link";
import type { ReactNode } from "react";

const contributionPalette = ["#ebe7dc", "#f6c7b5", "#ee9672", "#d76849", "#81362a"];

type HeroProps = {
  projects: ProjectCard[];
  languages: LanguageStats;
  activity: CommitActivity;
};

export function Hero({ projects, languages, activity }: HeroProps) {
  const reduce = useReducedMotion();
  const totalStars = projects.reduce((sum, project) => sum + project.stars, 0);

  return (
    <section className="section pb-10 pt-20">
      <div className="container grid items-center gap-10 lg:grid-cols-[1.05fr_0.95fr]">
        <motion.div
          className="max-w-3xl"
          initial={reduce ? false : { opacity: 0, y: 24 }}
          animate={reduce ? undefined : { opacity: 1, y: 0 }}
          transition={{ duration: 0.75, ease: [0.22, 1, 0.36, 1] }}
        >
          <p className="eyebrow mb-4 text-sm font-semibold uppercase">GitHub Portfolio Hub</p>
          <motion.div
            className="accent-rule mb-5"
            initial={reduce ? false : { scaleX: 0, transformOrigin: "left" }}
            animate={reduce ? undefined : { scaleX: 1 }}
            transition={{ duration: 0.8, delay: 0.12, ease: [0.22, 1, 0.36, 1] }}
          />
          <h1 className="text-5xl font-black leading-tight md:text-7xl">{siteName}</h1>
          <p className="mt-5 max-w-2xl text-xl leading-8 text-[var(--muted)]">
            {siteDescription}
          </p>
          <div className="mt-8 flex flex-wrap gap-3">
            <Link className="btn btn-primary" href="/projects">
              查看项目 <ArrowRight size={18} />
            </Link>
            <a className="btn btn-secondary" href={blogURL}>
              <BookOpen size={18} /> 访问博客
            </a>
          </div>
        </motion.div>

        <motion.div
          className="glass-hero min-h-[460px] overflow-hidden p-0"
          initial={reduce ? false : { opacity: 0, y: 20, rotateX: 8 }}
          animate={reduce ? undefined : { opacity: 1, y: 0, rotateX: 0 }}
          transition={{ duration: 0.9, delay: 0.16, ease: [0.22, 1, 0.36, 1] }}
          whileHover={reduce ? undefined : { y: -3 }}
        >
          <div className="absolute inset-0">
            <div className="absolute inset-x-8 top-10 h-px bg-[linear-gradient(90deg,transparent,var(--accent-warm),transparent)] opacity-40" />
            <div className="absolute bottom-10 left-8 right-8 h-px bg-[linear-gradient(90deg,transparent,var(--accent-leaf),transparent)] opacity-30" />
            <motion.div
              className="absolute left-[-18%] top-0 h-full w-[18%] bg-[linear-gradient(90deg,transparent,rgba(255,255,255,0.25),transparent)]"
              animate={reduce ? undefined : { x: ["0%", "620%"] }}
              transition={{ duration: 8, repeat: Infinity, ease: "linear", delay: 0.4 }}
            />
          </div>

          <div className="relative z-10 border-b border-[var(--line)] bg-[color-mix(in_srgb,var(--panel)_48%,transparent)] p-5">
            <div className="flex items-center justify-between">
              <span className="text-sm font-semibold text-[var(--muted)]">Recent commit activity</span>
              <Github size={20} />
            </div>
            <div className="mt-4 flex flex-wrap items-center gap-3 text-sm">
              <span className="font-bold text-[var(--foreground)]">{activity.totalContributions}</span>
              <span className="text-[var(--muted)]">contributions in the last year</span>
              <span className="text-[var(--muted)]">github/contribution-calendar</span>
            </div>
          </div>

          <div className="relative z-10 p-6">
            <ContributionHeatmap activity={activity} reduce={Boolean(reduce)} />

            <div className="mt-7 grid grid-cols-3 gap-px overflow-hidden rounded-lg border border-[var(--line)] bg-[var(--line)] text-sm">
              <Metric icon={<GitPullRequest size={16} />} label="项目" value={projects.length.toString()} />
              <Metric icon={<Star size={16} />} label="Stars" value={totalStars.toString()} />
              <Metric icon={<Code2 size={16} />} label="语言" value={languages.languages.length.toString()} />
            </div>
          </div>
        </motion.div>
      </div>
    </section>
  );
}

function ContributionHeatmap({ activity, reduce }: { activity: CommitActivity; reduce: boolean }) {
  const weeks = activity.weeks.slice(-32);
  const max = Math.max(activity.maxContributions, ...weeks.flatMap((week) => week.days.map((day) => day.count)), 0);
  const labels = monthLabels(weeks);

  return (
    <div className="rounded-lg border border-[var(--line)] bg-[color-mix(in_srgb,var(--panel)_58%,transparent)] p-4 shadow-[inset_0_1px_0_rgb(255_255_255_/_0.32)]">
      <div className="mb-4 flex items-end justify-between gap-4">
        <div>
          <div className="text-lg font-black leading-none">Commit 方块图</div>
          <div className="mt-2 text-xs text-[var(--muted)]">最近 {weeks.length} 周公开贡献记录</div>
        </div>
        <div className="hidden items-center gap-1 text-[10px] text-[var(--muted)] sm:flex">
          Less
          {contributionPalette.map((color) => (
            <span
              className="h-2.5 w-2.5 rounded-[2px] border border-[color-mix(in_srgb,var(--line)_70%,transparent)]"
              key={color}
              style={{ background: color }}
            />
          ))}
          More
        </div>
      </div>

      <div className="overflow-x-auto pb-2">
        <div className="w-max min-w-full">
          <div
            className="ml-8 grid gap-[3px] text-[10px] leading-none text-[var(--muted)]"
            style={{ gridTemplateColumns: `repeat(${weeks.length}, 9px)` }}
          >
            {weeks.map((week, index) => (
              <span className="h-4 whitespace-nowrap" key={week.firstDay}>
                {labels[index] ?? ""}
              </span>
            ))}
          </div>
          <div className="flex gap-2">
            <div className="grid grid-rows-7 gap-[3px] pt-[1px] text-[10px] leading-[9px] text-[var(--muted)]">
              <span />
              <span>Mon</span>
              <span />
              <span>Wed</span>
              <span />
              <span>Fri</span>
              <span />
            </div>
            <div
              className="grid grid-flow-col grid-rows-7 gap-[3px]"
              style={{ gridTemplateColumns: `repeat(${weeks.length}, 9px)` }}
            >
              {weeks.flatMap((week, weekIndex) =>
                normalizedDays(week).map((day, dayIndex) => {
                  const label = `${day.date}: ${day.count} contributions`;
                  return (
                    <motion.div
                      aria-label={label}
                      className="h-[9px] w-[9px]"
                      initial={reduce ? false : { opacity: 0, scale: 0.72 }}
                      animate={reduce ? undefined : { opacity: 1, scale: 1 }}
                      transition={{
                        duration: 0.22,
                        delay: Math.min(0.55, (weekIndex * 7 + dayIndex) * 0.004),
                        ease: "easeOut",
                      }}
                      whileHover={reduce ? undefined : { scale: 1.8, zIndex: 20 }}
                      key={`${week.firstDay}-${day.date}`}
                    >
                      {day.count > 0 ? (
                        <Link
                          aria-label={`${day.date}: ${day.count} contributions，查看详情`}
                          className="block h-[9px] w-[9px] rounded-[2px] border border-[color-mix(in_srgb,var(--line)_55%,transparent)] shadow-[inset_0_1px_0_rgb(255_255_255_/_0.28)] outline-none transition-shadow focus-visible:ring-2 focus-visible:ring-[var(--accent-warm)]"
                          href={`/activity?date=${day.date}`}
                          style={{ background: contributionColor(day.count, max) }}
                          title={label}
                        />
                      ) : (
                        <span
                          className="block h-[9px] w-[9px] rounded-[2px] border border-[color-mix(in_srgb,var(--line)_55%,transparent)] shadow-[inset_0_1px_0_rgb(255_255_255_/_0.28)]"
                          style={{ background: contributionColor(day.count, max) }}
                          title={label}
                        />
                      )}
                    </motion.div>
                  );
                }),
              )}
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}

function Metric({ icon, label, value }: { icon: ReactNode; label: string; value: string }) {
  return (
    <div className="bg-[color-mix(in_srgb,var(--panel)_58%,transparent)] p-3">
      <div className="flex items-center gap-2 text-lg font-bold">
        <span className="text-[var(--accent-warm)]">{icon}</span>
        {value}
      </div>
      <div className="mt-1 text-xs text-[var(--muted)]">{label}</div>
    </div>
  );
}

function normalizedDays(week: ContributionWeek) {
  return week.days;
}

function monthLabels(weeks: ContributionWeek[]) {
  return weeks.reduce<Record<number, string>>((labels, week, index) => {
    const current = new Date(`${week.firstDay}T00:00:00Z`);
    const previous = index > 0 ? new Date(`${weeks[index - 1].firstDay}T00:00:00Z`) : null;
    if (index === 0 || !previous || current.getUTCMonth() !== previous.getUTCMonth()) {
      labels[index] = current.toLocaleString("en-US", { month: "short" });
    }
    return labels;
  }, {});
}

function contributionColor(count: number, max: number) {
  if (count <= 0 || max <= 0) {
    return contributionPalette[0];
  }
  const ratio = count / max;
  if (ratio < 0.25) {
    return contributionPalette[1];
  }
  if (ratio < 0.5) {
    return contributionPalette[2];
  }
  if (ratio < 0.75) {
    return contributionPalette[3];
  }
  return contributionPalette[4];
}
