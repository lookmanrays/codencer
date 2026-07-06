import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";
import { Providers } from "@/components/providers";
import { DeviceApprovalPanel } from "@/components/console/device-approval-panel";

describe("DeviceApprovalPanel", () => {
  it("validates and approves in explicit demo mode without issuing tokens", async () => {
    const user = userEvent.setup();
    render(
      <Providers>
        <DeviceApprovalPanel />
      </Providers>,
    );
    await user.click(screen.getByRole("button", { name: /approve device/i }));
    expect(
      await screen.findByText(/enter a device code like ABCD-EFGH/i),
    ).toBeInTheDocument();
    await user.type(screen.getByLabelText(/user code/i), "ABCD-EFGH");
    await user.click(screen.getByRole("button", { name: /approve device/i }));
    expect(
      await screen.findByText(/device login approved/i),
    ).toBeInTheDocument();
    expect(screen.queryByText(/access_token/i)).not.toBeInTheDocument();
  });
});
