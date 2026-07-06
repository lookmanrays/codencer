"use client";

import { zodResolver } from "@hookform/resolvers/zod";
import { ShieldCheck } from "lucide-react";
import { useSearchParams } from "next/navigation";
import { useForm } from "react-hook-form";
import { z } from "zod";
import { useOAuthConsent, useOAuthConsentDecision } from "@/api/oauth";
import { isDemoMode } from "@/api/config";
import { Alert } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { EmptyState } from "@/components/ui/empty-state";
import { Field } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { KeyValueList } from "@/components/ui/key-value-list";
import { LoadingPanel } from "@/components/ui/skeleton";

const OperatorCodeSchema = z.object({
  operatorCode: z.string().min(6, "Approval code is required"),
});

type OperatorCode = z.infer<typeof OperatorCodeSchema>;

export function OAuthConsentPanel() {
  const search = useSearchParams();
  const consent = useOAuthConsent(
    search.toString() ? `?${search.toString()}` : "",
  );
  const decision = useOAuthConsentDecision();
  const {
    formState: { errors },
    handleSubmit,
    register,
  } = useForm<OperatorCode>({
    resolver: zodResolver(OperatorCodeSchema),
    defaultValues: { operatorCode: "" },
  });
  const request = consent.data?.request;

  return (
    <Card className="mx-auto max-w-[720px]">
      <CardHeader>
        <CardTitle className="flex items-center gap-sm">
          <ShieldCheck aria-hidden="true" className="h-5 w-5 text-accent" />
          Codencer Gateway OAuth consent
        </CardTitle>
        <p className="m-0 mt-xs text-body-sm text-ink-secondary">
          This page mirrors OAuth dev consent. Production provider login remains
          private/future.
        </p>
      </CardHeader>
      <CardContent>
        {isDemoMode() ? (
          <Alert title="Demo mode" tone="warning">
            OAuth consent is simulated only because demo mode is explicit.
          </Alert>
        ) : null}
        {consent.isLoading ? <LoadingPanel /> : null}
        {consent.error ? (
          <Alert title="OAuth request unavailable" tone="danger">
            {consent.error.message}
          </Alert>
        ) : null}
        {request && (!request.redirectURI || !request.codeChallenge) ? (
          <EmptyState
            description="Open this page from an OAuth authorization request with redirect_uri and PKCE parameters."
            title="No OAuth authorization request"
          />
        ) : null}
        {request && request.redirectURI && request.codeChallenge ? (
          <>
            <KeyValueList
              items={[
                { label: "Client", value: request.clientName },
                { label: "Workspace", value: request.workspaceId },
                { label: "Resource", value: request.resource },
                {
                  label: "Scopes",
                  value: (
                    <span className="flex flex-wrap gap-xs">
                      {request.scopes.map((scope) => (
                        <Badge key={scope}>{scope}</Badge>
                      ))}
                    </span>
                  ),
                },
              ]}
            />
            {decision.error ? (
              <Alert title="OAuth consent failed" tone="danger">
                {decision.error.message}
              </Alert>
            ) : null}
          </>
        ) : null}
        {decision.data ? (
          <div className="mt-md rounded-[var(--radius-card)] border border-border bg-paper-tinted p-md">
            <p className="m-0 font-semibold">
              Consent {decision.data.decision}.
            </p>
            <p className="mb-0 mt-xs text-body-sm text-ink-secondary">
              Redirect target: <code>{decision.data.redirect_uri}</code>
            </p>
          </div>
        ) : request && request.redirectURI && request.codeChallenge ? (
          <form
            className="mt-md grid gap-md"
            onSubmit={handleSubmit((values) =>
              decision.mutate({
                decision: "approve",
                operatorCode: values.operatorCode,
                request,
              }),
            )}
          >
            <Field
              error={errors.operatorCode?.message}
              id="operator-code"
              label="Operator approval code"
            >
              <Input
                id="operator-code"
                type="password"
                {...register("operatorCode")}
              />
            </Field>
            <div className="flex flex-wrap gap-sm">
              <Button disabled={decision.isPending} type="submit">
                Approve
              </Button>
              <Button
                disabled={decision.isPending}
                onClick={() =>
                  decision.mutate({
                    decision: "deny",
                    request,
                  })
                }
                type="button"
                variant="danger"
              >
                Deny
              </Button>
            </div>
          </form>
        ) : null}
      </CardContent>
    </Card>
  );
}
