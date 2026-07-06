"use client";

import { zodResolver } from "@hookform/resolvers/zod";
import { CheckCircle2 } from "lucide-react";
import { useForm } from "react-hook-form";
import { useDeviceApproval } from "@/api/device";
import { isDemoMode } from "@/api/config";
import { Alert } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Field } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import {
  DeviceApprovalInputSchema,
  type DeviceApprovalInput,
} from "@/schemas/device";

export function DeviceApprovalPanel() {
  const approval = useDeviceApproval();
  const {
    formState: { errors },
    handleSubmit,
    register,
  } = useForm<DeviceApprovalInput>({
    resolver: zodResolver(DeviceApprovalInputSchema),
    defaultValues: { userCode: "" },
  });

  return (
    <Card className="mx-auto max-w-[560px]">
      <CardHeader>
        <CardTitle>Approve Codencer device login</CardTitle>
        <p className="m-0 mt-xs text-body-sm text-ink-secondary">
          This form submits the device code to Gateway{" "}
          <code>/api/gateway/v1/device/approve</code>.
        </p>
      </CardHeader>
      <CardContent>
        {isDemoMode() ? (
          <Alert title="Demo mode" tone="warning">
            Device approval is simulated only because demo mode is explicit.
          </Alert>
        ) : null}
        {approval.error ? (
          <Alert title="Device approval failed" tone="danger">
            {approval.error.message}
          </Alert>
        ) : null}
        {approval.data ? (
          <div className="rounded-[var(--radius-card)] border border-success p-md">
            <CheckCircle2 aria-hidden="true" className="h-6 w-6 text-success" />
            <p className="mb-0 mt-sm font-semibold">
              Device login approved for {approval.data.workspace.name}.
            </p>
            <p className="mb-0 mt-xs text-body-sm text-ink-secondary">
              The polling CLI can now receive its Gateway token.
            </p>
          </div>
        ) : (
          <form
            className="grid gap-md"
            onSubmit={handleSubmit((values) => approval.mutate(values))}
          >
            <Field
              error={errors.userCode?.message}
              id="user-code"
              label="User code"
            >
              <Input
                autoComplete="one-time-code"
                id="user-code"
                placeholder="ABCD-EFGH"
                {...register("userCode")}
              />
            </Field>
            <Button disabled={approval.isPending} type="submit">
              {approval.isPending ? "Approving..." : "Approve device"}
            </Button>
          </form>
        )}
      </CardContent>
    </Card>
  );
}
