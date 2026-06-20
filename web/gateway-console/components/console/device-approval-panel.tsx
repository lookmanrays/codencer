"use client";

import { zodResolver } from "@hookform/resolvers/zod";
import { CheckCircle2 } from "lucide-react";
import { useState } from "react";
import { useForm } from "react-hook-form";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Field } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { DeviceCodeSchema, type DeviceCode } from "@/schemas/console";

export function DeviceApprovalPanel() {
  const [approved, setApproved] = useState(false);
  const {
    formState: { errors },
    handleSubmit,
    register,
  } = useForm<DeviceCode>({
    resolver: zodResolver(DeviceCodeSchema),
    defaultValues: { userCode: "" },
  });

  return (
    <Card className="mx-auto max-w-[560px]">
      <CardHeader>
        <CardTitle>Approve Codencer device login</CardTitle>
        <p className="m-0 mt-xs text-body-sm text-ink-secondary">
          Demo mode validates the approval form locally. A production deployment
          should submit to Gateway <code>/api/gateway/v1/device/approve</code>.
        </p>
      </CardHeader>
      <CardContent>
        {approved ? (
          <div className="rounded-[var(--radius-card)] border border-success p-md">
            <CheckCircle2 aria-hidden="true" className="h-6 w-6 text-success" />
            <p className="mb-0 mt-sm font-semibold">
              Device approval form validated.
            </p>
            <p className="mb-0 mt-xs text-body-sm text-ink-secondary">
              No token was created in mock mode.
            </p>
          </div>
        ) : (
          <form
            className="grid gap-md"
            onSubmit={handleSubmit(() => setApproved(true))}
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
            <Button type="submit">Approve device</Button>
          </form>
        )}
      </CardContent>
    </Card>
  );
}
