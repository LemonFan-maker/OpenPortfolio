"use client";

import { getProjectSummary } from "@/lib/api";
import type { ProjectSummaryStatus } from "@/lib/types";
import { Loader2, Sparkles } from "lucide-react";
import { useEffect, useState } from "react";

type ProjectAiSummaryProps = {
  repoName: string;
};

export function ProjectAiSummary({ repoName }: ProjectAiSummaryProps) {
  const [summary, setSummary] = useState<ProjectSummaryStatus | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  useEffect(() => {
    let ignore = false;

    async function loadSummary() {
      setLoading(true);
      setError("");
      setSummary(null);

      try {
        const nextSummary = await getProjectSummary(repoName);
        if (!ignore) {
          setSummary(nextSummary);
        }
      } catch (nextError) {
        if (!ignore) {
          setError(nextError instanceof Error ? nextError.message : "AI 简介加载失败");
          setSummary(null);
        }
      } finally {
        if (!ignore) {
          setLoading(false);
        }
      }
    }

    void loadSummary();
    return () => {
      ignore = true;
    };
  }, [repoName]);

  return (
    <section className="surface p-4">
      <div className="flex items-start justify-between gap-3">
        <div>
          <p className="text-xs uppercase tracking-[0.18em] text-[var(--muted)]">ai summary</p>
          <h3 className="mt-1 text-xl font-black">AI 简介</h3>
        </div>
        {loading ? <Loader2 className="animate-spin text-[var(--muted)]" size={18} /> : <Sparkles className="text-[var(--accent-warm)]" size={18} />}
      </div>

      <div className="mt-4 space-y-4">
        {loading ? (
          <div className="rounded-lg border border-[var(--line)] bg-[color-mix(in_srgb,var(--panel)_72%,transparent)] p-4">
            <div className="space-y-2">
              <div className="h-4 w-3/5 rounded bg-[color-mix(in_srgb,var(--line)_60%,transparent)]" />
              <div className="h-4 w-11/12 rounded bg-[color-mix(in_srgb,var(--line)_45%,transparent)]" />
              <div className="h-4 w-4/6 rounded bg-[color-mix(in_srgb,var(--line)_45%,transparent)]" />
            </div>
          </div>
        ) : error ? (
          <div className="rounded-lg border border-[var(--line)] bg-[color-mix(in_srgb,var(--panel)_72%,transparent)] p-4">
            <p className="text-sm leading-6 text-[var(--accent-warm)]">{error}</p>
          </div>
        ) : summary ? (
          <>
            {summary.source === "fallback" ? (
              <div className="rounded-lg border border-[var(--accent-warm)] bg-[color-mix(in_srgb,var(--accent-warm)_10%,var(--panel))] p-3">
                <p className="text-sm font-semibold leading-6 text-[var(--accent-warm)]">
                  AI 简介暂未生成成功，下面是根据仓库元数据生成的备用摘要。
                </p>
                {summary.error ? (
                  <p className="mt-1 break-words text-xs leading-5 text-[var(--muted)]">{summary.error}</p>
                ) : null}
              </div>
            ) : null}

            <div className="rounded-lg border border-[var(--line)] bg-[color-mix(in_srgb,var(--panel)_72%,transparent)] px-3 py-2">
              <p className="text-xs font-black uppercase tracking-[0.14em] text-[var(--muted)]">来源</p>
              <p className="mt-1 text-sm font-semibold text-[var(--foreground)]">{summary.source}</p>
            </div>

            <div className="rounded-lg border border-[var(--line)] bg-[color-mix(in_srgb,var(--panel)_72%,transparent)] p-4">
              <p className="text-xs font-black uppercase tracking-[0.14em] text-[var(--muted)]">一句话简介</p>
              <p className="mt-2 break-words text-sm font-semibold leading-6 text-[var(--foreground)]">
                {summary.summary.shortSummary}
              </p>
            </div>

            <div>
              <p className="text-xs font-black uppercase tracking-[0.14em] text-[var(--muted)]">详细简介</p>
              <p className="mt-2 break-words text-sm leading-6 text-[var(--muted)] whitespace-pre-wrap">
                {summary.summary.longSummary}
              </p>
            </div>

            {summary.summary.highlights.length > 0 ? (
              <div>
                <p className="text-xs font-black uppercase tracking-[0.14em] text-[var(--muted)]">亮点</p>
                <ul className="mt-2 space-y-2">
                  {summary.summary.highlights.slice(0, 3).map((item) => (
                    <li
                      className="flex gap-2 rounded-lg border border-[var(--line)] bg-[color-mix(in_srgb,var(--panel)_72%,transparent)] px-3 py-2 text-sm leading-6"
                      key={item}
                    >
                      <span className="mt-[0.55rem] h-1.5 w-1.5 shrink-0 rounded-full bg-[var(--accent-warm)]" />
                      <span className="break-words text-[var(--muted)]">{item}</span>
                    </li>
                  ))}
                </ul>
              </div>
            ) : null}
          </>
        ) : (
          <div className="rounded-lg border border-[var(--line)] bg-[color-mix(in_srgb,var(--panel)_72%,transparent)] p-4">
            <p className="text-sm leading-6 text-[var(--muted)]">暂无 AI 简介</p>
          </div>
        )}
      </div>
    </section>
  );
}
