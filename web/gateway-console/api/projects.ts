"use client";

import { useQuery } from "@tanstack/react-query";
import { isDemoMode } from "@/api/config";
import { gatewayJSON } from "@/api/http";
import { queryKeys } from "@/api/query-keys";
import { demoSnapshot } from "@/api/demo-data";
import {
  ProjectListResponseSchema,
  ProjectResponseSchema,
} from "@/schemas/projects";

export async function listProjects() {
  if (isDemoMode()) {
    return { projects: demoSnapshot.projects, relayErrors: [] };
  }
  return gatewayJSON("/projects", ProjectListResponseSchema);
}

export async function getProject(projectId: string) {
  if (isDemoMode()) {
    const project = demoSnapshot.projects.find((item) => item.id === projectId);
    if (!project) throw new Error(`Demo project ${projectId} not found`);
    return { project, relayErrors: [] };
  }
  return gatewayJSON(
    `/projects/${encodeURIComponent(projectId)}`,
    ProjectResponseSchema,
  );
}

export function useProjects() {
  return useQuery({
    queryKey: queryKeys.projects,
    queryFn: listProjects,
  });
}

export function useProjectLocations() {
  return useProjects();
}

export function useProject(projectId: string) {
  return useQuery({
    queryKey: queryKeys.project(projectId),
    queryFn: () => getProject(projectId),
    enabled: Boolean(projectId),
  });
}
