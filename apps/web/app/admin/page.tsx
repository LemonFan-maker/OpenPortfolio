import { AdminDashboard } from "@/components/admin/admin-dashboard";
import { getCommitActivity, getLanguageStats, getProjects } from "@/lib/api";

export default async function AdminPage() {
  const [projects, languages, activity] = await Promise.all([
    getProjects({ sort: "stars" }),
    getLanguageStats(),
    getCommitActivity(),
  ]);

  return <AdminDashboard activity={activity} languages={languages} projects={projects} />;
}
