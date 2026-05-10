import { ProjectBrowser } from "@/components/projects/project-browser";
import { getProjects } from "@/lib/api";

export default async function ProjectsPage() {
  const projects = await getProjects({ sort: "stars" });

  return (
    <section className="section">
      <div className="container">
        <div className="mb-8">
          <h1 className="text-4xl font-black">项目</h1>
          <p className="mt-3 max-w-2xl leading-7 text-[var(--muted)]">
            展示 GitHub 公开仓库，支持搜索、语言筛选和排序。
          </p>
        </div>
        <ProjectBrowser projects={projects} />
      </div>
    </section>
  );
}
