"use client";

import { getCommitActivityDetail } from "@/lib/api";
import type {
  CommitActivity,
  CommitActivityDetail,
  CommitFileDiff,
  CommitRecord,
  ContributionDay,
  ContributionWeek,
} from "@/lib/types";
import { motion, useReducedMotion } from "framer-motion";
import { ExternalLink } from "lucide-react";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";

const contributionPalette = ["#ebe7dc", "#f6c7b5", "#ee9672", "#d76849", "#81362a"];

type CommitActivityExplorerProps = {
  activity: CommitActivity;
  initialDate: string;
  initialDetail: CommitActivityDetail | null;
};

export function CommitActivityExplorer({ activity, initialDate, initialDetail }: CommitActivityExplorerProps) {
  const reduce = useReducedMotion();
  const weeks = activity.weeks;
  const max = Math.max(activity.maxContributions, ...weeks.flatMap((week) => week.days.map((day) => day.count)), 0);
  const labels = useMemo(() => monthLabels(weeks), [weeks]);
  const countByDate = useMemo(() => dayCountMap(weeks), [weeks]);
  const detailCache = useRef(new Map<string, CommitActivityDetail | null>([[initialDate, initialDetail]]));
  const autoLoadRef = useRef<string | null>(initialDetail ? initialDate : null);
  const activeCellRef = useRef<HTMLButtonElement | null>(null);
  const [activeDate, setActiveDate] = useState(initialDate);
  const activeDay = useMemo(() => findDayByDate(weeks, activeDate), [activeDate, weeks]);
  const [detail, setDetail] = useState<CommitActivityDetail | null>(initialDetail);
  const [selectedCommit, setSelectedCommit] = useState<CommitRecord | null>(initialDetail?.commits?.[0] ?? null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");

  const selectDay = useCallback(async (day: ContributionDay) => {
    setActiveDate(day.date);
    setError("");
    if (typeof window !== "undefined") {
      window.history.replaceState(null, "", `/activity?date=${day.date}`);
    }

    if (day.count <= 0) {
      setDetail(null);
      setSelectedCommit(null);
      setLoading(false);
      return;
    }

    if (detailCache.current.has(day.date)) {
      const cached = detailCache.current.get(day.date) ?? null;
      if (cached || day.count <= 0) {
        setDetail(cached);
        setSelectedCommit(cached?.commits?.[0] ?? null);
        setLoading(false);
        return;
      }
    }

    setLoading(true);
    const nextDetail = await getCommitActivityDetail(day.date);
    if (nextDetail) {
      detailCache.current.set(day.date, nextDetail);
    }
    setDetail(nextDetail);
    setSelectedCommit(nextDetail?.commits?.[0] ?? null);
    setError(nextDetail ? "" : "未能读取当天 commit 详情");
    setLoading(false);
  }, []);

  useEffect(() => {
    activeCellRef.current?.scrollIntoView({
      behavior: reduce ? "auto" : "smooth",
      block: "nearest",
      inline: "center",
    });
  }, [activeDate, reduce]);

  useEffect(() => {
    if (
      !initialDetail &&
      activeDay &&
      activeDay.count > 0 &&
      !detail &&
      !loading &&
      autoLoadRef.current !== activeDate
    ) {
      autoLoadRef.current = activeDate;
      void selectDay(activeDay);
    }
  }, [activeDay, detail, initialDetail, loading, selectDay]);

  return (
    <div className="space-y-6">
      <section className="surface p-5">
        <div className="mb-5 flex flex-wrap items-end justify-between gap-4">
          <div>
            <h2 className="text-2xl font-black">Commit Activity</h2>
            <p className="mt-2 text-sm text-[var(--muted)]">
              {activity.totalContributions} contributions in the last year
            </p>
          </div>
          <div className="flex items-center gap-1 text-xs text-[var(--muted)]">
            Less
            {contributionPalette.map((color) => (
              <span
                className="h-3.5 w-3.5 rounded-[3px] border border-[color-mix(in_srgb,var(--line)_70%,transparent)]"
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
              className="ml-10 grid gap-1 text-xs leading-none text-[var(--muted)]"
              style={{ gridTemplateColumns: `repeat(${weeks.length}, 14px)` }}
            >
              {weeks.map((week, index) => (
                <span className="h-5 whitespace-nowrap" key={week.firstDay}>
                  {labels[index] ?? ""}
                </span>
              ))}
            </div>
            <div className="flex gap-3">
              <div className="grid grid-rows-7 gap-1 pt-[1px] text-xs leading-[14px] text-[var(--muted)]">
                <span />
                <span>Mon</span>
                <span />
                <span>Wed</span>
                <span />
                <span>Fri</span>
                <span />
              </div>
              <div
                className="grid grid-flow-col grid-rows-7 gap-1"
                style={{ gridTemplateColumns: `repeat(${weeks.length}, 14px)` }}
              >
                {weeks.flatMap((week, weekIndex) =>
                  normalizedDays(week).map((day, dayIndex) => {
                    const isActive = day.date === activeDate;
                    return (
                      <motion.button
                        aria-label={`${day.date}: ${day.count} contributions`}
                        className={
                          isActive
                            ? "h-3.5 w-3.5 rounded-[3px] border border-[var(--foreground)] outline-none ring-2 ring-[color-mix(in_srgb,var(--accent-warm)_42%,transparent)]"
                            : "h-3.5 w-3.5 rounded-[3px] border border-[color-mix(in_srgb,var(--line)_62%,transparent)] outline-none transition focus-visible:ring-2 focus-visible:ring-[var(--accent-warm)]"
                        }
                        disabled={loading}
                        initial={reduce ? false : { opacity: 0, scale: 0.78 }}
                        animate={reduce ? undefined : { opacity: 1, scale: 1 }}
                        transition={{
                          duration: 0.2,
                          delay: Math.min(0.5, (weekIndex * 7 + dayIndex) * 0.002),
                          ease: "easeOut",
                        }}
                        key={`${week.firstDay}-${day.date}`}
                        ref={isActive ? activeCellRef : undefined}
                        onClick={() => void selectDay(day)}
                        style={{ background: contributionColor(day.count, max) }}
                        title={`${day.date}: ${day.count} contributions`}
                        type="button"
                      />
                    );
                  }),
                )}
              </div>
            </div>
          </div>
        </div>
      </section>

      <section className="grid items-start gap-5 lg:grid-cols-[360px_1fr]">
        <CommitList
          activeCount={countByDate.get(activeDate) ?? 0}
          date={activeDate}
          detail={detail}
          error={error}
          loading={loading}
          onSelectCommit={setSelectedCommit}
          selectedCommit={selectedCommit}
        />
        <CommitDiffView commit={selectedCommit} />
      </section>
    </div>
  );
}

function CommitList({
  activeCount,
  date,
  detail,
  error,
  loading,
  onSelectCommit,
  selectedCommit,
}: {
  activeCount: number;
  date: string;
  detail: CommitActivityDetail | null;
  error: string;
  loading: boolean;
  onSelectCommit: (commit: CommitRecord) => void;
  selectedCommit: CommitRecord | null;
}) {
  const commits = detail?.commits ?? [];

  return (
    <aside className="surface h-fit self-start p-4">
      <div className="mb-4">
        <h2 className="text-xl font-black">{date}</h2>
        <p className="mt-1 text-sm text-[var(--muted)]">{activeCount} contributions on calendar</p>
      </div>

      {loading ? <div className="rounded-md border border-[var(--line)] p-3 text-sm text-[var(--muted)]">加载 commit...</div> : null}
      {error ? <div className="rounded-md border border-[color-mix(in_srgb,var(--accent-warm)_45%,var(--line))] p-3 text-sm text-[var(--accent-warm)]">{error}</div> : null}
      {!loading && !error && activeCount <= 0 ? <div className="rounded-md border border-dashed border-[var(--line)] p-3 text-sm text-[var(--muted)]">这一天没有公开提交记录。</div> : null}
      {!loading && !error && activeCount > 0 && commits.length === 0 ? (
        <div className="rounded-md border border-dashed border-[var(--line)] p-3 text-sm text-[var(--muted)]">
          这一天有贡献记录，但没有可展示的 commit。可能是 PR、issue 或 review 贡献。
        </div>
      ) : null}

      <div className="max-h-[520px] space-y-2 overflow-y-auto pr-1">
        {commits.map((commit) => (
          <button
            className={
              selectedCommit?.sha === commit.sha
                ? "w-full rounded-md border border-[color-mix(in_srgb,var(--accent-warm)_50%,var(--line))] bg-[color-mix(in_srgb,var(--accent-warm)_14%,var(--panel))] p-3 text-left"
                : "w-full rounded-md border border-[var(--line)] bg-[color-mix(in_srgb,var(--panel)_64%,transparent)] p-3 text-left transition hover:border-[color-mix(in_srgb,var(--accent-warm)_42%,var(--line))]"
            }
            key={commit.sha}
            onClick={() => onSelectCommit(commit)}
            type="button"
          >
            <div className="flex items-center justify-between gap-2">
              <span className="truncate text-sm font-bold">{commit.repository}</span>
              <span className="shrink-0 font-mono text-[11px] text-[var(--muted)]">{commit.sha.slice(0, 7)}</span>
            </div>
            <div className="mt-2 line-clamp-2 text-sm leading-6 text-[var(--muted)]">{commit.message}</div>
            <div className="mt-3 flex items-center gap-3 text-xs font-semibold">
              <span className="text-[var(--accent-leaf)]">+{commit.additions}</span>
              <span className="text-red-700">-{commit.deletions}</span>
              <span className="text-[var(--muted)]">{commit.filesChanged} files</span>
            </div>
          </button>
        ))}
        {detail?.truncated ? <div className="px-1 text-xs text-[var(--muted)]">当天 commit 较多，这里只展示前几条。</div> : null}
      </div>
    </aside>
  );
}

function CommitDiffView({ commit }: { commit: CommitRecord | null }) {
  if (!commit) {
    return (
      <article className="surface flex min-h-[360px] items-center justify-center p-6 text-sm text-[var(--muted)]">
        点击上方有颜色的 commit 方块，再选择左侧 commit 查看 diff。
      </article>
    );
  }

  const files = commit.files ?? [];

  return (
    <article className="surface h-fit min-w-0 self-start p-0">
      <div className="flex flex-wrap items-start justify-between gap-3 border-b border-[var(--line)] p-5">
        <div className="min-w-0">
          <h2 className="truncate text-2xl font-black">{commit.message}</h2>
          <a className="mt-2 inline-flex max-w-full items-center gap-2 truncate text-sm text-[var(--muted)]" href={commit.url} rel="noreferrer" target="_blank">
            <ExternalLink size={14} />
            <span className="truncate">
              {commit.repository}@{commit.sha.slice(0, 7)}
            </span>
          </a>
        </div>
        <div className="flex shrink-0 gap-2 text-sm font-bold">
          <span className="rounded-md bg-[color-mix(in_srgb,var(--accent-leaf)_18%,var(--panel))] px-2 py-1 text-[var(--accent-leaf)]">+{commit.additions}</span>
          <span className="rounded-md bg-red-50 px-2 py-1 text-red-700">-{commit.deletions}</span>
        </div>
      </div>

      <div className="max-h-[680px] overflow-y-auto p-5">
        {files.length > 0 ? (
          files.map((file) => <FileDiff file={file} key={file.filename} />)
        ) : (
          <div className="rounded-md border border-dashed border-[var(--line)] p-4 text-sm text-[var(--muted)]">这个 commit 没有可展示的文件 diff。</div>
        )}
      </div>
    </article>
  );
}

function FileDiff({ file }: { file: CommitFileDiff }) {
  return (
    <section className="mb-5 overflow-hidden rounded-md border border-[var(--line)] last:mb-0">
      <div className="flex flex-wrap items-center justify-between gap-2 border-b border-[var(--line)] bg-[color-mix(in_srgb,var(--panel)_78%,transparent)] px-3 py-2">
        <div className="min-w-0 truncate font-mono text-sm font-bold">{file.filename}</div>
        <div className="flex shrink-0 items-center gap-2 text-xs font-semibold">
          <span className={statusClass(file.status)}>{file.status}</span>
          <span className="text-[var(--accent-leaf)]">+{file.additions}</span>
          <span className="text-red-700">-{file.deletions}</span>
        </div>
      </div>
      {file.patch ? (
        <pre className="overflow-x-auto bg-[color-mix(in_srgb,var(--foreground)_5%,transparent)] p-0 text-xs leading-5">
          <code>
            {file.patch.split("\n").map((line, index) => (
              <span className={`block min-w-max px-3 ${diffLineClass(line)}`} key={`${file.filename}-${index}`}>
                {line || " "}
              </span>
            ))}
          </code>
        </pre>
      ) : (
        <div className="p-3 text-sm text-[var(--muted)]">这个文件没有可展示的文本 patch。</div>
      )}
    </section>
  );
}

function normalizedDays(week: ContributionWeek) {
  if (week.days.length === 7) {
    return week.days;
  }
  return Array.from({ length: 7 }, (_, index) => {
    return week.days[index] ?? { date: `${week.firstDay}-${index}`, count: 0, color: contributionPalette[0] };
  });
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

function dayCountMap(weeks: ContributionWeek[]) {
  const map = new Map<string, number>();
  weeks.forEach((week) => {
    week.days.forEach((day) => map.set(day.date, day.count));
  });
  return map;
}

function findDayByDate(weeks: ContributionWeek[], date: string) {
  for (const week of weeks) {
    for (const day of week.days) {
      if (day.date === date) {
        return day;
      }
    }
  }
  return null;
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

function statusClass(status: string) {
  switch (status) {
    case "added":
      return "rounded bg-[color-mix(in_srgb,var(--accent-leaf)_18%,var(--panel))] px-2 py-0.5 text-[var(--accent-leaf)]";
    case "removed":
      return "rounded bg-red-50 px-2 py-0.5 text-red-700";
    default:
      return "rounded bg-[color-mix(in_srgb,var(--accent-gold)_26%,var(--panel))] px-2 py-0.5 text-[color-mix(in_srgb,var(--accent-warm)_82%,var(--foreground))]";
  }
}

function diffLineClass(line: string) {
  if (line.startsWith("@@")) {
    return "bg-[color-mix(in_srgb,var(--accent-gold)_36%,var(--panel))] font-semibold text-[color-mix(in_srgb,var(--accent-warm)_78%,var(--foreground))]";
  }
  if (line.startsWith("+++") || line.startsWith("---")) {
    return "bg-[color-mix(in_srgb,var(--accent-gold)_18%,var(--panel))] text-[color-mix(in_srgb,var(--accent-warm)_74%,var(--foreground))]";
  }
  if (line.startsWith("+")) {
    return "bg-[color-mix(in_srgb,var(--accent-leaf)_20%,var(--panel))] text-[color-mix(in_srgb,var(--accent-leaf)_74%,var(--foreground))]";
  }
  if (line.startsWith("-")) {
    return "bg-red-50 text-red-700";
  }
  return "text-[var(--foreground)]";
}
