#!/usr/bin/env bash
set -euo pipefail

usage() {
  echo "Usage: $0 --manifest cli.json --version VERSION [--artifacts DIR]" >&2
  exit 1
}

manifest=""
version=""
artifacts=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    --manifest)
      manifest="${2:-}"
      shift 2
      ;;
    --version)
      version="${2:-}"
      shift 2
      ;;
    --artifacts)
      artifacts="${2:-}"
      shift 2
      ;;
    -h|--help)
      usage
      ;;
    *)
      echo "unknown argument: $1" >&2
      usage
      ;;
  esac
done

if [[ -z "$manifest" || -z "$version" ]]; then
  usage
fi

if [[ ! -f "$manifest" ]]; then
  echo "manifest not found: $manifest" >&2
  exit 1
fi

downloads_json="$(jq -c --arg version "$version" '.versions[$version].downloads // [] | .[]' "$manifest")"
if [[ -z "$downloads_json" ]]; then
  echo "no downloads found for version $version in $manifest" >&2
  exit 1
fi

failures=0
while IFS= read -r entry; do
  [[ -z "$entry" ]] && continue

  platform="$(echo "$entry" | jq -r '.platform')"
  arch="$(echo "$entry" | jq -r '.arch')"
  filename="$(echo "$entry" | jq -r '.filename')"
  want="$(echo "$entry" | jq -r '.sha256 // ""' | tr '[:upper:]' '[:lower:]' | tr -d '[:space:]')"
  url="$(echo "$entry" | jq -r '.download_url // ""')"

  label="${platform}/${arch} (${filename})"

  if [[ -z "$want" ]]; then
    echo "skip $label: empty sha256"
    continue
  fi
  if [[ ! "$want" =~ ^[0-9a-f]{64}$ ]]; then
    echo "FAIL $label: invalid sha256 $want" >&2
    failures=$((failures + 1))
    continue
  fi

  if [[ -n "$artifacts" ]]; then
    # Prefer arch-scoped layout: artifacts/{platform}/{arch}/{filename}
    local_path="${artifacts}/${platform}/${arch}/${filename}"
    if [[ ! -f "$local_path" ]]; then
      # Legacy flat layout fallback
      local_path="${artifacts}/${platform}/${filename}"
    fi
    if [[ -f "$local_path" ]]; then
      got_local="$(sha256sum "$local_path" | awk '{print $1}' | tr '[:upper:]' '[:lower:]')"
      if [[ "$got_local" != "$want" ]]; then
        echo "FAIL $label: local artifact sha256 mismatch got $got_local want $want" >&2
        failures=$((failures + 1))
        continue
      fi
      echo "ok $label: local artifact sha256 matches"
    else
      echo "warn $label: local artifact missing at ${artifacts}/${platform}/${arch}/${filename}"
    fi
  fi

  if [[ -z "$url" ]]; then
    echo "FAIL $label: empty download_url" >&2
    failures=$((failures + 1))
    continue
  fi

  # Cache-bust + retry: Spaces/CDN can briefly serve a prior object after overwrite.
  got_remote=""
  remote_ok=0
  for attempt in 1 2 3 4 5; do
    bust="${url}"
    if [[ "$bust" == *"?"* ]]; then
      bust="${bust}&nocache=${RANDOM}${attempt}$(date +%s)"
    else
      bust="${bust}?nocache=${RANDOM}${attempt}$(date +%s)"
    fi
    if got_remote="$(curl -fsSL "$bust" | sha256sum | awk '{print $1}' | tr '[:upper:]' '[:lower:]')"; then
      if [[ "$got_remote" == "$want" ]]; then
        remote_ok=1
        break
      fi
    fi
    echo "retry $label: remote sha256 got ${got_remote:-curl-failed} want $want (attempt ${attempt}/5)"
    sleep $((attempt * 2))
  done
  if [[ "$remote_ok" -ne 1 ]]; then
    echo "FAIL $label: remote sha256 mismatch got ${got_remote:-curl-failed} want $want ($url)" >&2
    failures=$((failures + 1))
    continue
  fi
  echo "ok $label: remote sha256 matches"
done < <(jq -c --arg version "$version" '.versions[$version].downloads // [] | .[]' "$manifest")

if [[ "$failures" -gt 0 ]]; then
  echo "manifest verification failed with $failures error(s)" >&2
  exit 1
fi

echo "manifest verification passed for version $version"
