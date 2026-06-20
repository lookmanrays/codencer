import { DeviceApprovalPanel } from "@/components/console/device-approval-panel";
import { PageHeader } from "@/components/layout/page-header";

export default function DevicePage() {
  return (
    <main className="min-h-dvh bg-paper px-[var(--container-pad)] py-xl" id="main-content">
      <PageHeader
        description="Approve a CLI device-code login. This standalone page shares the console visual system."
        kicker="Device login"
        title="Codencer approval"
      />
      <DeviceApprovalPanel />
    </main>
  );
}
