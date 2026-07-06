package defaults

import "strings"

const (
	SelfHostGatewayBaseURL = "http://127.0.0.1:19090"
	SelfHostRelayURL       = "http://127.0.0.1:8090"
	SelfHostConsoleURL     = "http://127.0.0.1:3000"

	GatewayURLEnv = "CODENCER_GATEWAY_URL"
	MCPURLEnv     = "CODENCER_MCP_URL"
	RelayURLEnv   = "CODENCER_RELAY_URL"
	ConsoleURLEnv = "CODENCER_CONSOLE_URL"
)

// These variables are intentionally settable through -ldflags -X for private
// official builds. Empty public/self-built values fall back to self-host URLs.
var (
	GatewayBaseURL = ""
	GatewayMCPURL  = ""
	RelayURL       = ""
	ConsoleURL     = ""
)

func DefaultGatewayBaseURL() string {
	return normalizeGatewayBaseURL(firstNonEmpty(GatewayBaseURL, SelfHostGatewayBaseURL))
}

func DefaultGatewayMCPURL() string {
	value := strings.TrimRight(strings.TrimSpace(firstNonEmpty(GatewayMCPURL, DefaultGatewayBaseURL()+"/mcp")), "/")
	if !strings.HasSuffix(value, "/mcp") {
		value += "/mcp"
	}
	return value
}

func DefaultRelayURL() string {
	return strings.TrimRight(strings.TrimSpace(firstNonEmpty(RelayURL, SelfHostRelayURL)), "/")
}

func DefaultConsoleURL() string {
	return strings.TrimRight(strings.TrimSpace(firstNonEmpty(ConsoleURL, SelfHostConsoleURL)), "/")
}

func normalizeGatewayBaseURL(value string) string {
	value = strings.TrimRight(strings.TrimSpace(value), "/")
	value = strings.TrimSuffix(value, "/mcp")
	return strings.TrimRight(value, "/")
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
