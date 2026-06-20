import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";
import { Providers } from "@/components/providers";
import { DeviceApprovalPanel } from "@/components/console/device-approval-panel";

describe("DeviceApprovalPanel", () => {
  it("validates and approves in mock mode without issuing tokens", async () => {
    const user = userEvent.setup();
    render(
      <Providers>
        <DeviceApprovalPanel />
      </Providers>,
    );
    await user.click(screen.getByRole("button", { name: /approve device/i }));
    expect(await screen.findByText(/too small/i)).toBeInTheDocument();
    await user.type(screen.getByLabelText(/user code/i), "ABCD-EFGH");
    await user.click(screen.getByRole("button", { name: /approve device/i }));
    expect(await screen.findByText(/form validated/i)).toBeInTheDocument();
    expect(screen.queryByText(/access_token/i)).not.toBeInTheDocument();
  });
});
