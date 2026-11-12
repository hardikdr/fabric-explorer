#!/bin/sh
set -eu
if [ "$#" -ne 2 ]; then
  echo "Usage: $0 /path/to/local-reader.kubeconfig explicit-context" >&2
  exit 2
fi
command -v kubectl >/dev/null 2>&1 || { echo "kubectl is required for the read-only preflight" >&2; exit 2; }
project_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
[ -f "$1" ] || { echo "The kubeconfig file is unavailable" >&2; exit 2; }
[ -n "$2" ] || { echo "An explicit context is required" >&2; exit 2; }
[ -x "$project_root/bin/fabric-explorer" ] || { echo "Build first: make build" >&2; exit 2; }
for resource in switches.sonic.networking.metal.ironcore.dev nodes.fabric.ironcore.dev interfaces.fabric.ironcore.dev cells.fabric.ironcore.dev; do
  if ! kubectl --kubeconfig="$1" --context="$2" --request-timeout=10s get customresourcedefinitions.apiextensions.k8s.io "$resource" -o name >/dev/null 2>&1; then
    echo "Required API missing, forbidden, or unavailable: $resource. No changes were made; use fixture mode for a demo." >&2
    exit 1
  fi
done
cd "$project_root"
exec ./bin/fabric-explorer --mode live --kubeconfig "$1" --context "$2" --listen 127.0.0.1:8102
