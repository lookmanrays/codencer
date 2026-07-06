import { z } from "zod";

export class GatewayAPIError extends Error {
  code: string;
  status: number;

  constructor(status: number, code: string, message: string) {
    super(message);
    this.name = "GatewayAPIError";
    this.code = code;
    this.status = status;
  }
}

const APIErrorSchema = z.object({
  code: z.string().optional(),
  error: z.string().optional(),
  message: z.string().optional(),
});

export async function gatewayJSON<T>(
  path: string,
  schema: z.ZodSchema<T>,
  init?: RequestInit,
): Promise<T> {
  const response = await fetch(`/api/gateway/v1${path}`, {
    ...init,
    cache: "no-store",
    headers: {
      Accept: "application/json",
      ...(init?.body ? { "Content-Type": "application/json" } : {}),
      ...(init?.headers ?? {}),
    },
  });
  const raw = await response.text();
  const json = raw ? JSON.parse(raw) : {};
  if (!response.ok) {
    const parsed = APIErrorSchema.safeParse(json);
    throw new GatewayAPIError(
      response.status,
      parsed.success
        ? (parsed.data.code ?? "gateway_api_error")
        : "gateway_api_error",
      parsed.success
        ? (parsed.data.message ??
            parsed.data.error ??
            `Gateway API ${path} returned ${response.status}`)
        : `Gateway API ${path} returned ${response.status}`,
    );
  }
  return schema.parse(json);
}
