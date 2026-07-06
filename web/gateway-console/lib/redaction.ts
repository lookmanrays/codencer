const secretPatterns = [
  /Bearer\s+[A-Za-z0-9._~+/=-]+/gi,
  /token=[^\s]+/gi,
  /client_secret=[^\s]+/gi,
  /enrollment[_-]?token["'=:\s]+[A-Za-z0-9._~+/=-]+/gi,
];

const absolutePathPatterns = [/\/Users\/[^\s"']+/g, /\/home\/[^\s"']+/g];

export function redactSecret(value: string) {
  return secretPatterns.reduce(
    (out, pattern) => out.replace(pattern, "[redacted]"),
    value,
  );
}

export function stripAbsolutePaths(value: string) {
  return absolutePathPatterns.reduce(
    (out, pattern) => out.replace(pattern, "<local-path>"),
    value,
  );
}

export function sanitizeForDisplay(value: string) {
  return stripAbsolutePaths(redactSecret(value));
}
