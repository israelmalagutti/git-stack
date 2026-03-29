"use strict";

const os = require("os");
const path = require("path");

const PLATFORM_MAP = {
  darwin: "darwin",
  linux: "linux",
  win32: "windows",
};

const ARCH_MAP = {
  x64: "amd64",
  arm64: "arm64",
};

function getPlatformInfo() {
  const platform = PLATFORM_MAP[process.platform];
  if (!platform) {
    throw new Error(
      `Unsupported platform: ${process.platform}. ` +
        `Supported: ${Object.keys(PLATFORM_MAP).join(", ")}`
    );
  }

  const arch = ARCH_MAP[process.arch];
  if (!arch) {
    throw new Error(
      `Unsupported architecture: ${process.arch}. ` +
        `Supported: ${Object.keys(ARCH_MAP).join(", ")}`
    );
  }

  const ext = platform === "windows" ? ".exe" : "";
  const archiveExt = platform === "windows" ? ".zip" : ".tar.gz";

  return { platform, arch, ext, archiveExt };
}

function getVersion() {
  const pkg = require("../package.json");
  return pkg.version;
}

function getDownloadUrl() {
  const { platform, arch, archiveExt } = getPlatformInfo();
  const version = getVersion();
  return `https://github.com/israelmalagutti/git-stack/releases/download/v${version}/gs-${platform}-${arch}${archiveExt}`;
}

function getBinaryPath() {
  const { ext } = getPlatformInfo();
  return path.join(__dirname, "..", "bin", `gs-binary${ext}`);
}

module.exports = { getPlatformInfo, getVersion, getDownloadUrl, getBinaryPath };
