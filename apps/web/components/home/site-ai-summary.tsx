"use client";

import { getSiteSummary } from "@/lib/api";
import type { SiteSummaryStatus } from "@/lib/types";
import { Loader2, Sparkles } from "lucide-react";
import { useEffect, useState } from "react";

export function SiteAiSummary() {
  const [summary, setSummary] = useState<SiteSummaryStatus | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  useEffect(() => {
    let ignore = false;

    async function loadSummary() {
      setLoading(true);
      setError("");
      setSummary(null);

      try {
        const nextSummary = await getSiteSummary();
        if (!ignore) {
          setSummary(nextSummary);
        }
      } catch (nextError) {
        if (!ignore) {
          setError(nextError instanceof Error ? nextError.message : "AI 履历加载失败");
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
  }, []);

  return (
    <section className="surface p-5">
      <div className="flex items-start justify-between gap-3">
        <div>
          <p className="eyebrow text-xs font-black uppercase">AI summary</p>
          <h2 className="mt-2 text-2xl font-black">{summary?.summary.title ?? "个人履历"}</h2>
          {summary?.summary.subtitle ? (
            <p className="mt-2 max-w-2xl text-sm leading-6 text-[var(--muted)]">{summary.summary.subtitle}</p>
          ) : (
            <p className="mt-2 max-w-2xl text-sm leading-6 text-[var(--muted)]">
              基于项目简介、语言结构和贡献记录生成的个人技术画像。
            </p>
          )}
        </div>
        {loading ? <Loader2 className="animate-spin text-[var(--muted)]" size={18} /> : <Sparkles className="text-[var(--accent-warm)]" size={18} />}
      </div>

      <div className="mt-5">
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
          <div className="space-y-5">
            {summary.source === "fallback" ? (
              <div className="rounded-lg border border-[var(--accent-warm)] bg-[color-mix(in_srgb,var(--accent-warm)_10%,var(--panel))] p-4">
                <p className="text-sm font-semibold leading-6 text-[var(--accent-warm)]">
                  AI 履历暂未生成成功，下面是根据项目元数据生成的备用摘要。
                </p>
                {summary.error ? (
                  <p className="mt-1 break-words text-xs leading-5 text-[var(--muted)]">{summary.error}</p>
                ) : null}
              </div>
            ) : null}

            <div className="rounded-lg border border-[var(--line)] bg-[color-mix(in_srgb,var(--panel)_72%,transparent)] p-4">
              <p className="break-words text-base font-semibold leading-7 text-[var(--foreground)]">
                {summary.summary.shortSummary}
              </p>
            </div>

            <p className="break-words whitespace-pre-wrap text-sm leading-7 text-[var(--muted)]">
              {summary.summary.longSummary}
            </p>

            {summary.summary.sections.length > 0 ? (
              <div className="grid gap-3 md:grid-cols-2">
                {summary.summary.sections.map((section) => (
                  <div className="rounded-lg border border-[var(--line)] bg-[color-mix(in_srgb,var(--panel)_72%,transparent)] p-4" key={section.title}>
                    <div className="text-sm font-black text-[var(--foreground)]">{section.title}</div>
                    {section.body ? (
                      <p className="mt-2 break-words whitespace-pre-wrap text-sm leading-6 text-[var(--muted)]">{section.body}</p>
                    ) : null}
                    {section.items?.length ? (
                      <ul className="mt-2 space-y-2">
                        {section.items.map((item) => (
                          <li className="flex gap-2 text-sm leading-6 text-[var(--muted)]" key={item}>
                            <span className="mt-[0.55rem] h-1.5 w-1.5 shrink-0 rounded-full bg-[var(--accent-warm)]" />
                            <span className="break-words">{item}</span>
                          </li>
                        ))}
                      </ul>
                    ) : null}
                  </div>
                ))}
              </div>
            ) : null}

            {summary.summary.techTags.length > 0 ? (
              <div className="flex flex-wrap gap-2">
                {summary.summary.techTags.map((tag) => (
                  <span
                    className="rounded-full border border-[var(--line)] bg-[color-mix(in_srgb,var(--panel)_72%,transparent)] px-3 py-1 text-xs font-semibold text-[var(--muted)]"
                    key={tag}
                  >
                    {tag}
                  </span>
                ))}
              </div>
            ) : null}
          </div>
        ) : (
          <div className="rounded-lg border border-[var(--line)] bg-[color-mix(in_srgb,var(--panel)_72%,transparent)] p-4">
            <p className="text-sm leading-6 text-[var(--muted)]">AI 履历暂未生成成功</p>
          </div>
        )}
      </div>
    </section>
  );
}
