import { DeviceApprovalPanel } from "@/components/console/device-approval-panel";
import { AuthShell } from "@/components/layout/auth-shell";

export default function DevicePage() {
  return (
    <AuthShell
      description="Approve a CLI device-code login from a controlled Gateway authorization page."
      kicker="Device login"
      title="Codencer approval"
    >
      <DeviceApprovalPanel />
    </AuthShell>
  );
}
