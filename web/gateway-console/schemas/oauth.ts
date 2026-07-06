import { z } from "zod";

export const OAuthConsentRequestSchema = z.object({
  clientId: z.string(),
  clientName: z.string(),
  codeChallenge: z.string(),
  codeChallengeMethod: z.string(),
  redirectURI: z.string(),
  resource: z.string(),
  responseType: z.string(),
  scope: z.string(),
  scopes: z.array(z.string()),
  state: z.string(),
  workspaceId: z.string(),
});

export const OAuthConsentResponseSchema = z
  .object({
    request: z.object({
      client_id: z.string(),
      client_name: z.string(),
      code_challenge: z.string(),
      code_challenge_method: z.string(),
      redirect_uri: z.string(),
      resource: z.string(),
      response_type: z.string(),
      scope: z.string(),
      scopes: z.array(z.string()),
      state: z.string(),
      workspace_id: z.string(),
    }),
  })
  .transform(({ request }) => ({
    request: OAuthConsentRequestSchema.parse({
      clientId: request.client_id,
      clientName: request.client_name,
      codeChallenge: request.code_challenge,
      codeChallengeMethod: request.code_challenge_method,
      redirectURI: request.redirect_uri,
      resource: request.resource,
      responseType: request.response_type,
      scope: request.scope,
      scopes: request.scopes,
      state: request.state,
      workspaceId: request.workspace_id,
    }),
  }));

export const OAuthConsentDecisionInputSchema = z.object({
  decision: z.enum(["approve", "deny"]),
  operatorCode: z.string().optional(),
  request: OAuthConsentRequestSchema,
});

export const OAuthConsentDecisionResponseSchema = z.object({
  authorization_code_issued: z.boolean().optional(),
  decision: z.enum(["approved", "denied"]),
  ok: z.boolean(),
  redirect_uri: z.string(),
});

export type OAuthConsentRequest = z.infer<typeof OAuthConsentRequestSchema>;
export type OAuthConsentDecisionInput = z.infer<
  typeof OAuthConsentDecisionInputSchema
>;
export type OAuthConsentDecisionResponse = z.infer<
  typeof OAuthConsentDecisionResponseSchema
>;
