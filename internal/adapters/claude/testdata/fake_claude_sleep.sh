#!/bin/sh
set -eu

trap 'exit 0' TERM INT
cat >/dev/null
while true; do
  sleep 1
done
