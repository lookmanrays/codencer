"use client";

import { zodResolver } from "@hookform/resolvers/zod";
import { useForm } from "react-hook-form";
import { z } from "zod";
import { Button } from "@/components/ui/button";
import { Field } from "@/components/ui/field";
import { Input } from "@/components/ui/input";

const RelayProfileFormSchema = z.object({
  name: z.string().min(2, "Name is required"),
  url: z.string().url("Use a valid Relay URL"),
  tokenEnv: z.string().min(2, "Use an environment variable reference"),
});

type RelayProfileFormValues = z.infer<typeof RelayProfileFormSchema>;

export function RelayProfileForm() {
  const {
    formState: { errors },
    handleSubmit,
    register,
  } = useForm<RelayProfileFormValues>({
    resolver: zodResolver(RelayProfileFormSchema),
    defaultValues: {
      name: "Personal self-host Relay",
      url: "https://relay.example.com",
      tokenEnv: "CODENCER_RELAY_PERSONAL_TOKEN",
    },
  });

  return (
    <form
      className="grid min-w-0 max-w-full gap-md rounded-[var(--radius-card)] border border-border bg-paper-strong p-md"
      onSubmit={handleSubmit(() => undefined)}
    >
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
      <Button type="submit">Validate form</Button>
    </form>
  );
}
