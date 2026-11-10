#!/bin/sh
set -eu
# Qdrant treats an empty API_KEY as enabled authentication. An omitted optional
# key must be truly absent, while a configured key is preserved unchanged.
if [ -z "${QDRANT__SERVICE__API_KEY:-}" ]; then
  unset QDRANT__SERVICE__API_KEY
fi
exec ./entrypoint.sh
