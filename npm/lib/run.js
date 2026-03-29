"use strict";

const { execFileSync } = require("child_process");
const fs = require("fs");
const { getBinaryPath } = require("./platform");

async function ensureBinary() {
  const binaryPath = getBinaryPath();
  if (fs.existsSync(binaryPath)) {
    return binaryPath;
  }

  // Binary not found — try downloading (handles npx case where postinstall may be skipped)
  console.error("gs binary not found, downloading...");
  const { install } = require("./install.js");
  await install();

  if (!fs.existsSync(binaryPath)) {
    console.error(
      "Failed to download gs binary. Install manually:\n" +
        "  curl -fsSL https://raw.githubusercontent.com/israelmalagutti/git-stack/main/scripts/install.sh | sh"
    );
    process.exit(1);
  }

  return binaryPath;
}

async function main() {
  const binaryPath = await ensureBinary();
  const args = process.argv.slice(2);

  try {
    execFileSync(binaryPath, args, { stdio: "inherit" });
  } catch (err) {
    if (err.status != null) {
      process.exitCode = err.status;
    } else if (err.signal) {
      process.kill(process.pid, err.signal);
    } else {
      process.exitCode = 1;
    }
  }
}

main();
