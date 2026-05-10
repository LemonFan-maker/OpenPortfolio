"use client";

import type { LanguageStats } from "@/lib/types";

const colors = ["#1F2421", "#2A9D8F", "#E76F51", "#F4A261", "#6A994E", "#C17C3A", "#4F772D", "#8A5A44", "#DDA15E"];
const center = 100;
const outerRadius = 76;
const ringGap = 10;
const strokeWidth = 7;

export function LanguageChart({ stats }: { stats: LanguageStats }) {
  const data = stats.languages.map((language, index) => ({
    name: language.name,
    percent: language.percent,
    bytes: language.bytes,
    fill: colors[index % colors.length],
  }));
  const visible = data.slice(0, 7);
  const dominant = data[0];

  return (
    <div className="flex min-h-[430px] w-full flex-col">
      <div className="mb-4 flex items-start justify-between gap-4">
        <div>
          <p className="text-xs uppercase tracking-[0.18em] text-[var(--muted)]">code footprint</p>
          <h3 className="mt-1 text-2xl font-black">{dominant?.name ?? "No data"}</h3>
        </div>
        <div className="text-right">
          <div className="text-3xl font-black">{formatBytes(stats.totalBytes)}</div>
          <div className="mt-1 text-xs text-[var(--muted)]">tracked bytes</div>
        </div>
      </div>

      <div className="flex flex-1 flex-col">
        <div className="flex justify-center py-4">
          <svg className="h-[250px] w-[250px]" viewBox="0 0 200 200" role="img" aria-label="Language distribution concentric rings">
            <defs>
              <filter id="languageRingShadow" x="-20%" y="-20%" width="140%" height="140%">
                <feDropShadow dx="0" dy="2" stdDeviation="2" floodColor="rgb(31 36 33 / 0.12)" />
              </filter>
            </defs>

            {visible.map((item, index) => {
              const radius = outerRadius - index * ringGap;
              const circumference = 2 * Math.PI * radius;
              const dashOffset = circumference * (1 - item.percent / 100);
              return (
                <g key={item.name} filter="url(#languageRingShadow)">
                  <circle
                    cx={center}
                    cy={center}
                    r={radius}
                    fill="none"
                    stroke="color-mix(in srgb, var(--line) 62%, transparent)"
                    strokeWidth={strokeWidth}
                  />
                  <circle
                    cx={center}
                    cy={center}
                    r={radius}
                    fill="none"
                    stroke={item.fill}
                    strokeDasharray={circumference}
                    strokeDashoffset={dashOffset}
                    strokeLinecap="round"
                    strokeWidth={strokeWidth}
                    transform={`rotate(-90 ${center} ${center})`}
                  />
                </g>
              );
            })}
          </svg>
        </div>

        <div className="max-h-[180px] space-y-2 overflow-y-auto pr-1">
          {data.map((item, index) => (
            <div className="rounded-lg border border-[var(--line)] bg-[color-mix(in_srgb,var(--panel)_66%,transparent)] px-3 py-2" key={item.name}>
              <div className="flex items-center gap-2">
                <span className="h-2.5 w-2.5 shrink-0 rounded-full" style={{ background: item.fill }} />
                <span className="min-w-0 flex-1 truncate text-sm font-semibold text-[var(--foreground)]">{item.name}</span>
                <span className="text-sm font-bold text-[var(--foreground)]">{item.percent}%</span>
              </div>
              <div className="mt-2 h-1.5 overflow-hidden rounded-full bg-[color-mix(in_srgb,var(--line)_70%,transparent)]">
                <div
                  className="h-full rounded-full"
                  style={{
                    width: `${Math.max(3, item.percent)}%`,
                    background: item.fill,
                    opacity: index === 0 ? 1 : 0.82,
                  }}
                />
              </div>
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}

function formatBytes(bytes: number) {
  if (bytes < 1024) {
    return `${bytes} B`;
  }
  if (bytes < 1024 * 1024) {
    return `${(bytes / 1024).toFixed(1)} KB`;
  }
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}
