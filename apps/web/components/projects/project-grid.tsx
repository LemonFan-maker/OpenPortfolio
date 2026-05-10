import type { ProjectCard as ProjectCardType } from "@/lib/types";
import { ProjectCard } from "./project-card";

export function ProjectGrid({ projects }: { projects: ProjectCardType[] }) {
  if (projects.length === 0) {
    return (
      <div className="rounded-lg border border-dashed border-[var(--line)] bg-[color-mix(in_srgb,var(--panel)_58%,transparent)] p-6 text-sm text-[var(--muted)]">
        没有找到匹配的项目。
      </div>
    );
  }

  return (
    <div className="columns-1 gap-5 md:columns-2 lg:columns-3">
      {projects.map((project) => (
        <ProjectCard key={project.name} project={project} />
      ))}
    </div>
  );
}
