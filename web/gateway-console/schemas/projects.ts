import { z } from "zod";
import { collectionField } from "@/schemas/collections";

export const ProjectLocationSchema = z.object({
  ambiguity: z.enum(["none", "relay_profile", "machine_location"]),
  connectorId: z.string(),
  hostLabel: z.string(),
  hostname: z.string(),
  id: z.string(),
  machineId: z.string(),
  projectId: z.string(),
  relayProfileId: z.string(),
  repoHash: z.string(),
  repoLabel: z.string(),
  status: z.enum(["online", "offline"]),
});

export const ProjectSchema = z.object({
  adapter: z.string(),
  id: z.string(),
  locations: z.array(ProjectLocationSchema),
  name: z.string(),
  profile: z.string(),
});

const RawProjectSchema = z.object({
  adapter_profile: z.string().optional(),
  default_adapter: z.string().optional(),
  name: z.string().optional(),
  project_id: z.string(),
  relay_profiles: collectionField(
    z.object({
      adapter_profile: z.string().optional(),
      default_adapter: z.string().optional(),
      locations: z
        .preprocess(
          (value) => value ?? [],
          z.array(
            z.object({
              connector_id: z.string().optional(),
              host_label: z.string().optional(),
              hostname: z.string().optional(),
              machine_id: z.string().optional(),
              online: z.boolean().optional(),
              repo_label: z.string().optional(),
              repo_root_hash: z.string().optional(),
              status: z.string().optional(),
            }),
          ),
        )
        .optional(),
      relay_profile_id: z.string(),
      status: z.string().optional(),
    }),
  ),
});

const RelayErrorSchema = z.record(z.string(), z.unknown());

export const ProjectListResponseSchema = z
  .object({
    projects: collectionField(RawProjectSchema),
    relay_errors: collectionField(RelayErrorSchema).optional(),
  })
  .transform(({ projects, relay_errors }) => ({
    projects: projects.map(normalizeProject),
    relayErrors: relay_errors ?? [],
  }));

export const ProjectResponseSchema = z
  .object({
    project: RawProjectSchema,
    relay_errors: collectionField(RelayErrorSchema).optional(),
  })
  .transform(({ project, relay_errors }) => ({
    project: normalizeProject(project),
    relayErrors: relay_errors ?? [],
  }));

function normalizeProject(project: z.infer<typeof RawProjectSchema>) {
  const multiRelay = project.relay_profiles.length > 1;
  const firstRelayWithExecutor = project.relay_profiles.find(
    (relay) => relay.adapter_profile || relay.default_adapter,
  );
  const adapter =
    project.default_adapter ?? firstRelayWithExecutor?.default_adapter;
  const profile =
    project.adapter_profile ?? firstRelayWithExecutor?.adapter_profile;
  const locations = project.relay_profiles.flatMap((relay) => {
    const relayLocations = relay.locations ?? [];
    const multiMachine =
      relayLocations.filter((location) => location.online).length > 1;
    return relayLocations.map((location, index) =>
      ProjectLocationSchema.parse({
        ambiguity: multiRelay
          ? "relay_profile"
          : multiMachine
            ? "machine_location"
            : "none",
        connectorId: location.connector_id ?? "",
        hostLabel: location.host_label ?? "",
        hostname: location.hostname ?? "",
        id: `${project.project_id}:${relay.relay_profile_id}:${location.machine_id ?? index}`,
        machineId: location.machine_id ?? "",
        projectId: project.project_id,
        relayProfileId: relay.relay_profile_id,
        repoHash: location.repo_root_hash ?? "",
        repoLabel: location.repo_label ?? project.project_id,
        status: location.online === false ? "offline" : "online",
      }),
    );
  });
  return ProjectSchema.parse({
    adapter: adapter ?? "project default",
    id: project.project_id,
    locations,
    name: project.name ?? project.project_id,
    profile: profile ?? adapter ?? "project default",
  });
}

export type Project = z.infer<typeof ProjectSchema>;
export type ProjectLocation = z.infer<typeof ProjectLocationSchema>;
