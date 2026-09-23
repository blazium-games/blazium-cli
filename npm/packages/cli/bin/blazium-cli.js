#!/usr/bin/env node
"use strict";

const { spawnSync } = require("child_process");
const fs = require("fs");
const path = require("path");

const platform = process.platform;
const arch = process.arch;
const pkgName = `@blazium-engine/cli-${platform}-${arch}`;
const exe = platform === "win32" ? "blazium-cli.exe" : "blazium-cli";

function fail(message) {
  console.error(message);
  process.exit(1);
}

let pkgJson;
try {
  pkgJson = require.resolve(`${pkgName}/package.json`);
} catch {
  fail(
    `No @blazium-engine/cli binary for ${platform}/${arch}. ` +
      "Supported: linux and win32, x64 and ia32."
  );
}

const bin = path.join(path.dirname(pkgJson), "bin", exe);
if (!fs.existsSync(bin)) {
  fail(`Missing binary ${bin}`);
}

const result = spawnSync(bin, process.argv.slice(2), { stdio: "inherit" });
if (result.error) {
  fail(result.error.message);
}
process.exit(result.status === null ? 1 : result.status);
