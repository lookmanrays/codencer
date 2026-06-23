import { RunDetailScreen } from "@/features/console/run-detail-screen";

export default async function RunDetailPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = await params;
  return <RunDetailScreen id={id} />;
}
