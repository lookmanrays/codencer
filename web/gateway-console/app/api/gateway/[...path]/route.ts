import { NextRequest, NextResponse } from "next/server";

const gatewayBase =
  process.env.CODENCER_GATEWAY_API_BASE ??
  process.env.CODENCER_GATEWAY_BASE_URL ??
  "http://127.0.0.1:19090";

const gatewayToken =
  process.env.CODENCER_GATEWAY_CONSOLE_TOKEN ??
  process.env.CODENCER_GATEWAY_MCP_TOKEN ??
  process.env.CODENCER_GATEWAY_TOKEN ??
  "";

type GatewayRouteContext = {
  params: Promise<{ path?: string[] }>;
};

export async function GET(request: NextRequest, context: GatewayRouteContext) {
  return proxyGateway(request, context, "GET");
}

export async function POST(request: NextRequest, context: GatewayRouteContext) {
  return proxyGateway(request, context, "POST");
}

export async function DELETE(
  request: NextRequest,
  context: GatewayRouteContext,
) {
  return proxyGateway(request, context, "DELETE");
}

async function proxyGateway(
  request: NextRequest,
  context: GatewayRouteContext,
  method: "GET" | "POST" | "DELETE",
) {
  const { path = [] } = await context.params;
  const upstreamURL = new URL(
    `/api/gateway/${path.map(encodeURIComponent).join("/")}`,
    gatewayBase,
  );
  upstreamURL.search = request.nextUrl.search;

  const headers = new Headers();
  headers.set("Accept", "application/json");
  if (method !== "GET") {
    headers.set(
      "Content-Type",
      request.headers.get("Content-Type") ?? "application/json",
    );
  }
  if (gatewayToken) {
    headers.set("Authorization", `Bearer ${gatewayToken}`);
  }

  let response: Response;
  try {
    response = await fetch(upstreamURL, {
      body: method === "GET" ? undefined : await request.text(),
      cache: "no-store",
      headers,
      method,
    });
  } catch {
    return NextResponse.json(
      {
        code: "gateway_unavailable",
        message: "Gateway API is unavailable in live mode.",
      },
      { status: 502 },
    );
  }

  const body = await response.text();
  const contentType =
    response.headers.get("Content-Type") ?? "application/json";
  return new NextResponse(body, {
    headers: {
      "Cache-Control": "no-store",
      "Content-Type": contentType,
    },
    status: response.status,
  });
}
