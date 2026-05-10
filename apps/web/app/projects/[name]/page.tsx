import { LanguageChart } from "@/components/charts/language-chart";
import { ProjectAiSummary } from "@/components/projects/project-ai-summary";
import { ReadmeMarkdown } from "@/components/projects/readme-markdown";
import { getProject } from "@/lib/api";
import { ExternalLink, GitFork, Star } from "lucide-react";
import type { ReactNode } from "react";
import { notFound } from "next/navigation";

export default async function ProjectDetailPage({ params }: { params: Promise<{ name: string }> }) {
  const { name } = await params;
  const project = await getProject(name);
  if (!project) {
    notFound();
  }

  return (
    <section className="section">
      <div className="container">
        <div className="mb-8 max-w-3xl">
          <h1 className="text-5xl font-black">{project.name}</h1>
          <p className="mt-4 text-xl leading-8 text-[var(--muted)]">{project.description}</p>
          <div className="mt-6 flex flex-wrap gap-3">
            <a className="btn btn-primary" href={project.url}>
              <ExternalLink size={18} /> GitHub
            </a>
            {project.homepage && (
              <a className="btn btn-secondary" href={project.homepage}>
                Demo
              </a>
            )}
          </div>
        </div>
        <div className="grid items-start gap-6 lg:grid-cols-[1fr_360px]">
          <article className="surface self-start p-6">
            <h2 className="text-2xl font-black">README 摘要</h2>
            <div className="mt-4">
              <ReadmeMarkdown content={project.readme} />
            </div>
          </article>
          <aside className="self-start space-y-4">
            <div className="surface p-5">
              <div className="grid grid-cols-3 gap-3 text-center text-sm text-[var(--muted)]">
                <StatItem icon={<Star size={16} />} label="Stars" value={project.stars.toString()} />
                <StatItem icon={<GitFork size={16} />} label="Forks" value={project.forks.toString()} />
                <StatItem icon={<span className="text-base leading-none">◎</span>} label="Issues" value={project.openIssues.toString()} />
              </div>
              <div className="mt-4 flex flex-wrap gap-2">
                {project.topics.map((topic) => (
                  <span className="rounded-md border border-[var(--line)] px-2 py-1 text-xs text-[var(--muted)]" key={topic}>
                    {topic}
                  </span>
                ))}
              </div>
            </div>
            <div className="surface p-4">
              <LanguageChart stats={{ totalBytes: project.languages.reduce((total, item) => total + item.bytes, 0), languages: project.languages }} />
            </div>
            <ProjectAiSummary repoName={project.name} />
          </aside>
        </div>
      </div>
    </section>
  );
}

function StatItem({ icon, label, value }: { icon: ReactNode; label: string; value: string }) {
  return (
    <div className="flex flex-col items-center justify-center gap-1 rounded-lg border border-[var(--line)] bg-[color-mix(in_srgb,var(--panel)_72%,transparent)] px-3 py-3">
      <div className="flex items-center gap-1.5 text-[var(--foreground)]">
        <span className="text-[var(--muted)]">{icon}</span>
        <span className="text-base font-semibold">{value}</span>
      </div>
      <div className="text-xs text-[var(--muted)]">{label}</div>
    </div>
  );
}
