"use client";

import { zodResolver } from "@hookform/resolvers/zod";
import { ShieldCheck } from "lucide-react";
import { useState } from "react";
import { useForm } from "react-hook-form";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Field } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { KeyValueList } from "@/components/ui/key-value-list";
import { OAuthConsentSchema, type OAuthConsent } from "@/schemas/console";

const defaultConsent: OAuthConsent = {
  clientId: "chatgpt-dev-client",
  clientName: "ChatGPT custom MCP app",
  workspaceId: "ws_personal",
  resource: "https://mcp.codencer.dev/mcp",
  scopes: ["projects:read", "projects:write"],
  operatorCode: "",
};

export function OAuthConsentPanel() {
  const [decision, setDecision] = useState<"approved" | "denied" | null>(null);
  const {
    formState: { errors },
    handleSubmit,
    register,
  } = useForm<OAuthConsent>({
    resolver: zodResolver(OAuthConsentSchema),
    defaultValues: defaultConsent,
  });

  return (
    <Card className="mx-auto max-w-[720px]">
      <CardHeader>
        <CardTitle className="flex items-center gap-sm">
          <ShieldCheck aria-hidden="true" className="h-5 w-5 text-accent" />
          Codencer Gateway OAuth consent
        </CardTitle>
        <p className="m-0 mt-xs text-body-sm text-ink-secondary">
          This page mirrors OAuth dev consent. Production provider login remains private/future.
        </p>
      </CardHeader>
      <CardContent>
        <KeyValueList
          items={[
            { label: "Client", value: defaultConsent.clientName },
            { label: "Workspace", value: defaultConsent.workspaceId },
            { label: "Resource", value: defaultConsent.resource },
            {
              label: "Scopes",
              value: (
                <span className="flex flex-wrap gap-xs">
                  {defaultConsent.scopes.map((scope) => (
                    <Badge key={scope}>{scope}</Badge>
                  ))}
                </span>
              ),
            },
          ]}
        />
        {decision ? (
          <div className="mt-md rounded-[var(--radius-card)] border border-border bg-paper-tinted p-md">
            <p className="m-0 font-semibold">Consent {decision} in mock mode.</p>
            <p className="mb-0 mt-xs text-body-sm text-ink-secondary">
              No authorization code or token was issued.
            </p>
          </div>
        ) : (
          <form
            className="mt-md grid gap-md"
            onSubmit={handleSubmit(() => setDecision("approved"))}
          >
            <Field
              error={errors.operatorCode?.message}
              id="operator-code"
              label="Operator approval code"
            >
              <Input id="operator-code" type="password" {...register("operatorCode")} />
            </Field>
            <div className="flex flex-wrap gap-sm">
              <Button type="submit">Approve</Button>
              <Button onClick={() => setDecision("denied")} type="button" variant="danger">
                Deny
              </Button>
            </div>
          </form>
        )}
      </CardContent>
    </Card>
  );
}
