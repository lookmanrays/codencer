"use client";

import { zodResolver } from "@hookform/resolvers/zod";
import { useForm } from "react-hook-form";
import { useCreateRelayProfile } from "@/api/relays";
import {
  CreateRelayProfileInputSchema,
  type CreateRelayProfileInput,
} from "@/schemas/relays";
import { Alert } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { Field } from "@/components/ui/field";
import { Input } from "@/components/ui/input";

export function RelayProfileForm() {
  const createRelay = useCreateRelayProfile();
  const {
    formState: { errors },
    handleSubmit,
    register,
    reset,
  } = useForm<CreateRelayProfileInput>({
    resolver: zodResolver(CreateRelayProfileInputSchema),
    defaultValues: {
      enabled: true,
      name: "Personal self-host Relay",
      url: "https://relay.example.com",
      tokenEnv: "CODENCER_RELAY_PERSONAL_TOKEN",
    },
  });

  return (
    <form
      className="grid min-w-0 max-w-full gap-md rounded-[var(--radius-card)] border border-border bg-paper-strong p-md"
      onSubmit={handleSubmit(async (values) => {
        await createRelay.mutateAsync(values);
        reset(values);
      })}
    >
      {createRelay.error ? (
        <Alert title="Relay profile was not saved" tone="error">
          {createRelay.error.message}
        </Alert>
      ) : null}
      {createRelay.isSuccess ? (
        <Alert title="Relay profile saved" tone="accent">
          Gateway persisted the Relay profile and refreshed the list.
        </Alert>
      ) : null}
      <Field error={errors.name?.message} id="relay-name" label="Profile name">
        <Input id="relay-name" {...register("name")} />
      </Field>
      <Field error={errors.url?.message} id="relay-url" label="Relay URL">
        <Input id="relay-url" {...register("url")} />
      </Field>
      <Field
        description="Store the real planner token on the Gateway host, not in browser state."
        error={errors.tokenEnv?.message}
        id="relay-token-env"
        label="Token environment variable"
      >
        <Input id="relay-token-env" {...register("tokenEnv")} />
      </Field>
      <Button disabled={createRelay.isPending} type="submit">
        {createRelay.isPending ? "Saving..." : "Save Relay profile"}
      </Button>
    </form>
  );
}
