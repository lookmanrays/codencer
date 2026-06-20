"use client";

import { useMutation, useQuery } from "@tanstack/react-query";
import { isDemoMode } from "@/api/config";
import { gatewayJSON } from "@/api/http";
import {
  OAuthConsentDecisionInputSchema,
  OAuthConsentDecisionResponseSchema,
  OAuthConsentResponseSchema,
  type OAuthConsentDecisionInput,
} from "@/schemas/oauth";

export async function getOAuthConsent(search: string) {
  if (isDemoMode()) {
    return {
      request: {
        clientId: "chatgpt-dev-client",
        clientName: "ChatGPT custom MCP app",
        codeChallenge: "demo-code-challenge",
        codeChallengeMethod: "S256",
        redirectURI: "http://127.0.0.1/callback",
        resource: "https://mcp.codencer.dev/mcp",
        responseType: "code",
        scope: "projects:read projects:write",
        scopes: ["projects:read", "projects:write"],
        state: "demo-state",
        workspaceId: "ws_personal",
      },
    };
  }
  return gatewayJSON(`/oauth/consent${search}`, OAuthConsentResponseSchema);
}

export async function decideOAuthConsent(input: OAuthConsentDecisionInput) {
  const values = OAuthConsentDecisionInputSchema.parse(input);
  if (isDemoMode()) {
    return {
      authorization_code_issued: values.decision === "approve",
      decision: values.decision === "approve" ? "approved" : "denied",
      ok: true,
      redirect_uri:
        values.decision === "approve"
          ? `${values.request.redirectURI}?code=demo-code&state=${values.request.state}`
          : `${values.request.redirectURI}?error=access_denied&state=${values.request.state}`,
    };
  }
  return gatewayJSON("/oauth/consent", OAuthConsentDecisionResponseSchema, {
    body: JSON.stringify({
      client_id: values.request.clientId,
      code_challenge: values.request.codeChallenge,
      code_challenge_method: values.request.codeChallengeMethod,
      decision: values.decision === "approve" ? "approve" : "deny",
      operator_code: values.operatorCode ?? "",
      redirect_uri: values.request.redirectURI,
      resource: values.request.resource,
      response_type: values.request.responseType,
      scope: values.request.scope,
      state: values.request.state,
    }),
    method: "POST",
  });
}

export function useOAuthConsent(search: string) {
  return useQuery({
    queryKey: ["oauth-consent", search],
    queryFn: () => getOAuthConsent(search),
  });
}

export function useOAuthConsentDecision() {
  return useMutation({
    mutationFn: decideOAuthConsent,
  });
}
