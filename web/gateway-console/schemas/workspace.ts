import { z } from "zod";

export const UserSchema = z.object({
  displayName: z.string(),
  email: z.string().email(),
  id: z.string(),
});

export const WorkspaceSchema = z.object({
  id: z.string(),
  kind: z.string(),
  mode: z.enum(["live", "demo"]),
  name: z.string(),
  slug: z.string(),
});

export const WorkspaceResponseSchema = z
  .object({
    mcp_url: z.string().url(),
    mode: z.string(),
    public_base_url: z.string().url(),
    user: z.object({
      display_name: z.string().optional(),
      email: z.string().email(),
      id: z.string(),
    }),
    workspace: z.object({
      id: z.string(),
      kind: z.string(),
      name: z.string(),
    }),
  })
  .transform((value) => ({
    mcpEndpoint: value.mcp_url,
    publicBaseURL: value.public_base_url,
    user: {
      displayName: value.user.display_name ?? value.user.email,
      email: value.user.email,
      id: value.user.id,
    },
    workspace: {
      id: value.workspace.id,
      kind: value.workspace.kind,
      mode: "live" as const,
      name: value.workspace.name,
      slug: value.workspace.id,
    },
  }));

export type User = z.infer<typeof UserSchema>;
export type Workspace = z.infer<typeof WorkspaceSchema>;
export type WorkspaceResponse = z.infer<typeof WorkspaceResponseSchema>;
