import { Suspense } from "react";
import { ProjectRunScreen } from "@/features/console/project-run-screen";
import { LoadingPanel } from "@/components/ui/skeleton";

export default function ProjectRunPage() {
  return (
    <Suspense fallback={<LoadingPanel />}>
      <ProjectRunScreen />
    </Suspense>
  );
}
