#!/usr/bin/env node
"use strict";

const { spawnSync } = require("child_process");
const crypto = require("crypto");
const fs = require("fs");
const http = require("http");
const https = require("https");
const os = require("os");
const path = require("path");

const CATALOG_URL = "https://cdn.blazium.app/cli/cli.json";

function fail(message) {
  console.error(message);
  process.exit(1);
}

function platformTarget(platform, arch) {
  const table = {
    "linux:x64": { cdnPlatform: "linux", cdnArch: "x86_64", exe: "blazium-cli" },
    "linux:ia32": { cdnPlatform: "linux", cdnArch: "x86_32", exe: "blazium-cli" },
    "win32:x64": { cdnPlatform: "windows", cdnArch: "x86_64", exe: "blazium-cli.exe" },
    "win32:ia32": { cdnPlatform: "windows", cdnArch: "x86_32", exe: "blazium-cli.exe" },
  };
  return table[`${platform}:${arch}`] || null;
}

function selectDownload(doc, version, cdnPlatform, cdnArch) {
  const entry = doc && doc.versions && doc.versions[version];
  if (!entry || !Array.isArray(entry.downloads)) {
    return null;
  }
  return (
    entry.downloads.find((item) => item.platform === cdnPlatform && item.arch === cdnArch) ||
    null
  );
}

function optionalBin(pkgName, exe) {
  let pkgJson;
  try {
    pkgJson = require.resolve(`${pkgName}/package.json`);
  } catch (err) {
    if (err && (err.code === "MODULE_NOT_FOUND" || /Cannot find module/.test(String(err.message)))) {
      return null;
    }
    throw err;
  }
  const bin = path.join(path.dirname(pkgJson), "bin", exe);
  if (!fs.existsSync(bin)) {
    fail(`Optional package ${pkgName} is installed but ${bin} is missing.`);
  }
  return bin;
}

function cacheFile(version, exe) {
  const base =
    process.platform === "win32"
      ? process.env.LOCALAPPDATA || path.join(os.homedir(), "AppData", "Local")
      : path.join(os.homedir(), ".cache");
  return path.join(base, "blazium", "npm-cli", version, exe);
}

function fetchBuffer(url, redirectsLeft) {
  return new Promise((resolve, reject) => {
    const lib = url.startsWith("http:") ? http : https;
    const req = lib.get(url, (res) => {
      const code = res.statusCode || 0;
      if (code >= 300 && code < 400 && res.headers.location) {
        res.resume();
        if (redirectsLeft <= 0) {
          reject(new Error(`Too many redirects for ${url}`));
          return;
        }
        resolve(fetchBuffer(new URL(res.headers.location, url).href, redirectsLeft - 1));
        return;
      }
      if (code !== 200) {
        res.resume();
        reject(new Error(`HTTP ${code} for ${url}`));
        return;
      }
      const chunks = [];
      res.on("data", (chunk) => chunks.push(chunk));
      res.on("end", () => resolve(Buffer.concat(chunks)));
    });
    req.on("error", reject);
  });
}

async function downloadBinary(spec, version, dest) {
  const catalog = JSON.parse((await fetchBuffer(CATALOG_URL, 5)).toString("utf8"));
  const download = selectDownload(catalog, version, spec.cdnPlatform, spec.cdnArch);
  if (!download || !download.download_url || !download.sha256) {
    fail(
      `@blazium-engine/cli ${version} has no ${spec.cdnPlatform}/${spec.cdnArch} download in ${CATALOG_URL}`
    );
  }
  const body = await fetchBuffer(download.download_url, 5);
  const got = crypto.createHash("sha256").update(body).digest("hex");
  if (got.toLowerCase() !== String(download.sha256).toLowerCase()) {
    fail(`Checksum mismatch for ${download.download_url}`);
  }
  fs.mkdirSync(path.dirname(dest), { recursive: true });
  const partial = `${dest}.partial`;
  fs.writeFileSync(partial, body);
  if (process.platform !== "win32") {
    fs.chmodSync(partial, 0o755);
  }
  fs.renameSync(partial, dest);
  return dest;
}

function run(bin) {
  const result = spawnSync(bin, process.argv.slice(2), { stdio: "inherit" });
  if (result.error) {
    fail(result.error.message);
  }
  process.exit(result.status === null ? 1 : result.status);
}

async function main() {
  const spec = platformTarget(process.platform, process.arch);
  if (!spec) {
    fail(
      `No @blazium-engine/cli binary for ${process.platform}/${process.arch}. ` +
        "Supported: linux and win32, x64 and ia32."
    );
  }

  const pkgName = `@blazium-engine/cli-${process.platform}-${process.arch}`;
  const installed = optionalBin(pkgName, spec.exe);
  if (installed) {
    run(installed);
    return;
  }

  const wrapper = require(path.join(__dirname, "..", "package.json"));
  const version = wrapper.version;
  const cached = cacheFile(version, spec.exe);
  if (fs.existsSync(cached)) {
    run(cached);
    return;
  }

  console.error(`Optional package ${pkgName} is not installed. Downloading Blazium CLI ${version}.`);
  run(await downloadBinary(spec, version, cached));
}

if (require.main === module) {
  main().catch((err) => fail(err && err.message ? err.message : String(err)));
}

module.exports = { platformTarget, selectDownload, cacheFile };
