import { redirect } from "next/navigation";

export default async function ActivityDateRedirect({ params }: { params: Promise<{ date: string }> }) {
  const { date } = await params;
  redirect(`/activity?date=${encodeURIComponent(date)}`);
}
