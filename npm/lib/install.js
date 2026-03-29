"use strict";

const https = require("https");
const fs = require("fs");
const path = require("path");
const { execSync } = require("child_process");
const { getPlatformInfo, getDownloadUrl, getBinaryPath } = require("./platform");

function followRedirects(url, maxRedirects = 5) {
  return new Promise((resolve, reject) => {
    if (maxRedirects === 0) {
      reject(new Error("Too many redirects"));
      return;
    }

    const proto = url.startsWith("https") ? https : require("http");
    proto
      .get(url, { headers: { "User-Agent": "gitstack-cli-npm" } }, (res) => {
        if (res.statusCode >= 300 && res.statusCode < 400 && res.headers.location) {
          followRedirects(res.headers.location, maxRedirects - 1)
            .then(resolve)
            .catch(reject);
          return;
        }

        if (res.statusCode !== 200) {
          reject(new Error(`Download failed: HTTP ${res.statusCode} from ${url}`));
          return;
        }

        resolve(res);
      })
      .on("error", reject);
  });
}

async function download(url, dest) {
  const res = await followRedirects(url);
  return new Promise((resolve, reject) => {
    const file = fs.createWriteStream(dest);
    res.pipe(file);
    file.on("finish", () => {
      file.close(resolve);
    });
    file.on("error", (err) => {
      fs.unlinkSync(dest);
      reject(err);
    });
  });
}

async function extractTarGz(archivePath, destDir) {
  execSync(`tar -xzf "${archivePath}" -C "${destDir}"`, { stdio: "pipe" });
}

async function extractZip(archivePath, destDir) {
  execSync(`unzip -o "${archivePath}" -d "${destDir}"`, { stdio: "pipe" });
}

async function install() {
  const binaryPath = getBinaryPath();

  // Skip if binary already exists
  if (fs.existsSync(binaryPath)) {
    return;
  }

  const { platform, arch, ext, archiveExt } = getPlatformInfo();
  const url = getDownloadUrl();
  const tmpDir = fs.mkdtempSync(path.join(require("os").tmpdir(), "gs-"));
  const archivePath = path.join(tmpDir, `gs${archiveExt}`);

  try {
    console.log(`Downloading gs binary from ${url}...`);
    await download(url, archivePath);

    // Extract archive
    if (archiveExt === ".zip") {
      await extractZip(archivePath, tmpDir);
    } else {
      await extractTarGz(archivePath, tmpDir);
    }

    // Find the gs binary in extracted files
    // GoReleaser archives contain "gs", older releases contain "gs-{os}-{arch}"
    const candidates = [
      path.join(tmpDir, `gs${ext}`),
      path.join(tmpDir, `gs-${platform}-${arch}${ext}`),
    ];
    const extractedBinary = candidates.find((p) => fs.existsSync(p));
    if (!extractedBinary) {
      throw new Error(`Binary not found in archive. Looked for: ${candidates.join(", ")}`);
    }

    // Ensure bin directory exists
    const binDir = path.dirname(binaryPath);
    if (!fs.existsSync(binDir)) {
      fs.mkdirSync(binDir, { recursive: true });
    }

    fs.copyFileSync(extractedBinary, binaryPath);

    if (platform !== "windows") {
      fs.chmodSync(binaryPath, 0o755);
    }

    console.log("gs binary installed successfully.");
  } finally {
    // Clean up temp directory
    fs.rmSync(tmpDir, { recursive: true, force: true });
  }
}

module.exports = { install };

// Run directly when called as postinstall script
if (require.main === module) {
  install().catch((err) => {
    console.warn(`Warning: Failed to install gs binary: ${err.message}`);
    console.warn("The binary will be downloaded on first run.");
    // Don't fail the npm install — run.js will retry at runtime
  });
}
