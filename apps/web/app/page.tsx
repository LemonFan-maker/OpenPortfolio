import { LanguageChart } from "@/components/charts/language-chart";
import { Reveal } from "@/components/animated/reveal";
import { Hero } from "@/components/home/hero";
import { SiteAiSummary } from "@/components/home/site-ai-summary";
import { Stats } from "@/components/home/stats";
import { ProjectGrid } from "@/components/projects/project-grid";
import { getCommitActivity, getLanguageStats, getProjects } from "@/lib/api";
import { ArrowRight } from "lucide-react";
import Link from "next/link";

export default async function HomePage() {
  const [projects, languages, activity] = await Promise.all([
    getProjects({ sort: "featured" }),
    getLanguageStats(),
    getCommitActivity(),
  ]);
  const featured = projects.slice(0, 3);

  return (
    <>
      <Hero projects={projects} languages={languages} activity={activity} />
      <Reveal>
        <Stats projects={projects} languages={languages} />
      </Reveal>
      <section className="section pt-0">
        <Reveal className="container">
          <SiteAiSummary />
        </Reveal>
      </section>
      <section className="section">
        <Reveal className="container">
          <div className="mb-6 flex items-end justify-between gap-4">
            <div>
              <h2 className="text-3xl font-black">精选项目</h2>
              <p className="mt-2 text-[var(--muted)]">按 Star 和活跃度展示重点仓库。</p>
            </div>
            <Link className="btn btn-secondary" href="/projects">
              全部项目 <ArrowRight size={18} />
            </Link>
          </div>
          <ProjectGrid projects={featured} />
        </Reveal>
      </section>
      <section className="section pt-0">
        <Reveal className="container grid gap-8 lg:grid-cols-[0.95fr_1.05fr]">
          <div>
            <h2 className="text-3xl font-black">语言占比</h2>
            <p className="mt-3 max-w-xl leading-7 text-[var(--muted)]">
              后端汇总公开仓库 languages API 结果，过滤 fork、归档和隐藏项目后计算占比。
            </p>
          </div>
          <div className="surface p-4">
            <LanguageChart stats={languages} />
          </div>
        </Reveal>
      </section>
    </>
  );
}
