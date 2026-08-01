#!/usr/bin/env bash
# Repair cli.json checksums by fetching each published binary from CDN.
# Use when manifest sha256 entries drift from uploaded artifacts.
set -euo pipefail

manifest_url="${CLI_MANIFEST_URL:-https://cdn.blazium.app/cli/cli.json}"
version="${CLI_REPAIR_VERSION:-}"
out="${CLI_REPAIR_OUT:-cli.json}"

manifest_file="$(mktemp)"
curl -fsSL "$manifest_url" -o "$manifest_file"

if [[ -z "$version" ]]; then
  version="$(jq -r '.latest // empty' "$manifest_file")"
fi
if [[ -z "$version" ]]; then
  echo "could not determine version; set CLI_REPAIR_VERSION" >&2
  exit 1
fi

echo "repairing version $version from $manifest_url"

mapfile -t downloads < <(jq -c --arg version "$version" '.versions[$version].downloads // [] | .[]' "$manifest_file")
if [[ "${#downloads[@]}" -eq 0 ]]; then
  echo "no downloads for version $version" >&2
  exit 1
fi

updated=0
for entry in "${downloads[@]}"; do
  platform="$(echo "$entry" | jq -r '.platform')"
  arch="$(echo "$entry" | jq -r '.arch')"
  filename="$(echo "$entry" | jq -r '.filename')"
  url="$(echo "$entry" | jq -r '.download_url // ""')"
  if [[ -z "$url" ]]; then
    echo "skip ${platform}/${arch}: empty download_url"
    continue
  fi

  blob="$(mktemp)"
  curl -fsSL "$url" -o "$blob"
  got="$(sha256sum "$blob" | awk '{print $1}' | tr '[:upper:]' '[:lower:]')"
  size="$(stat -c%s "$blob" 2>/dev/null || stat -f%z "$blob")"
  rm -f "$blob"

  want="$(echo "$entry" | jq -r '.sha256 // ""' | tr '[:upper:]' '[:lower:]' | tr -d '[:space:]')"
  if [[ "$got" == "$want" && "$(echo "$entry" | jq -r '.size // 0')" == "$size" ]]; then
    echo "ok ${platform}/${arch}: sha256 already matches"
    continue
  fi

  echo "fix ${platform}/${arch}: ${want:-<empty>} -> $got (size $size)"
  next="$(jq \
    --arg version "$version" \
    --arg platform "$platform" \
    --arg arch "$arch" \
    --arg sha256 "$got" \
    --argjson size "$size" \
    '
      .versions[$version].downloads = (
        .versions[$version].downloads
        | map(
            if (.platform == $platform and .arch == $arch)
            then .sha256 = $sha256 | .size = $size
            else .
            end
          )
      )
    ' "$manifest_file")"
  printf '%s\n' "$next" > "$manifest_file"
  updated=$((updated + 1))
done

cp "$manifest_file" "$out"
rm -f "$manifest_file"

if [[ "$updated" -eq 0 ]]; then
  echo "no checksum updates required; wrote $out"
else
  echo "updated $updated download(s); wrote $out"
fi

if command -v scripts/verify-cli-manifest.sh >/dev/null 2>&1 || [[ -x scripts/verify-cli-manifest.sh ]]; then
  scripts/verify-cli-manifest.sh --manifest "$out" --version "$version"
fi
