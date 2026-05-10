import { CommitActivityExplorer } from "@/components/activity/commit-activity-explorer";
import { getCommitActivity } from "@/lib/api";
import Link from "next/link";

export default async function ActivityPage({
  searchParams,
}: {
  searchParams: Promise<{ date?: string }>;
}) {
  const { date: rawDate } = await searchParams;
  const activity = await getCommitActivity();
  const initialDate = resolveInitialDate(activity, rawDate);

  return (
    <section className="section">
      <div className="container">
        <div className="mb-8 flex flex-wrap items-end justify-between gap-4">
          <div>
            <p className="eyebrow mb-3 text-sm font-semibold uppercase">GitHub Commit Timeline</p>
            <h1 className="text-4xl font-black md:text-5xl">提交记录详情</h1>
          </div>
          <Link className="btn btn-secondary" href="/">
            返回首页
          </Link>
        </div>
        <CommitActivityExplorer activity={activity} initialDate={initialDate} initialDetail={null} />
      </div>
    </section>
  );
}

function resolveInitialDate(activity: Awaited<ReturnType<typeof getCommitActivity>>, rawDate?: string) {
  if (rawDate && /^\d{4}-\d{2}-\d{2}$/.test(rawDate)) {
    return rawDate;
  }

  for (let weekIndex = activity.weeks.length - 1; weekIndex >= 0; weekIndex--) {
    const week = activity.weeks[weekIndex];
    for (let dayIndex = week.days.length - 1; dayIndex >= 0; dayIndex--) {
      const day = week.days[dayIndex];
      if (day.count > 0) {
        return day.date;
      }
    }
  }

  return activity.weeks.at(-1)?.days.at(-1)?.date ?? new Date().toISOString().slice(0, 10);
}
