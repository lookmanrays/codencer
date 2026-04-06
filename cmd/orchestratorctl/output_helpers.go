package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

type cliErrorPayload struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
	Status  int    `json:"status,omitempty"`
}

func emitJSONDocument(v any) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}

func emitJSONBody(body []byte) {
	var value any
	if err := json.Unmarshal(body, &value); err == nil {
		emitJSONDocument(value)
		return
	}
	emitJSONDocument(map[string]string{"value": string(body)})
}

func emitStructuredError(exitCode int, message string, details string) {
	payload := cliErrorPayload{
		Error:   message,
		Message: details,
	}
	emitJSONDocument(payload)
	os.Exit(exitCode)
}

func emitHumanError(exitCode int, message string, details string) {
	if details == "" {
		fmt.Fprintln(os.Stderr, message)
	} else {
		fmt.Fprintf(os.Stderr, "%s: %s\n", message, details)
	}
	os.Exit(exitCode)
}

func failCLI(jsonMode bool, exitCode int, message string, details string) {
	if jsonMode {
		emitStructuredError(exitCode, message, details)
	}
	emitHumanError(exitCode, message, details)
}

func failHTTP(jsonMode bool, status int, body []byte) {
	details := strings.TrimSpace(string(body))
	if details == "" {
		details = httpStatusText(status)
	}
	if jsonMode {
		emitJSONDocument(cliErrorPayload{
			Error:   "request_failed",
			Message: details,
			Status:  status,
		})
		os.Exit(exitCodeForHTTPStatus(status))
	}

	if details != "" {
		fmt.Fprintln(os.Stderr, details)
	}
	os.Exit(exitCodeForHTTPStatus(status))
}

func httpStatusText(status int) string {
	return strings.TrimSpace(fmt.Sprintf("request failed with status %d", status))
}

func hasFlag(args []string, flag string) bool {
	for _, arg := range args {
		if arg == flag {
			return true
		}
	}
	return false
}

func printJSON(body []byte) {
	var pretty bytes.Buffer
	if err := json.Indent(&pretty, body, "", "  "); err == nil {
		fmt.Println(pretty.String())
		return
	}
	fmt.Println(string(body))
}
