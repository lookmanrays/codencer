#!/bin/sh
set -eu

cat >/dev/null
printf 'claude stderr error\n' >&2
printf '{"type":"result","subtype":"error_during_execution","is_error":true,"errors":["Rate limit exceeded"]}\n'
exit 1
