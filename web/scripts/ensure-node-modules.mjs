import { spawnSync } from "node:child_process";
import { createHash } from "node:crypto";
import { existsSync, mkdirSync, readFileSync, writeFileSync } from "node:fs";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const scriptDir = dirname(fileURLToPath(import.meta.url));
const projectRoot = resolve(scriptDir, "..");
const manifestPath = join(projectRoot, "package.json");
const lockfilePath = join(projectRoot, "package-lock.json");
const nodeModulesPath = join(projectRoot, "node_modules");
const markerPath = join(nodeModulesPath, ".package-manifest.hash");

function readText(filePath) {
  return readFileSync(filePath, "utf8");
}

function readManifest() {
  return JSON.parse(readText(manifestPath));
}

function expectedHash() {
  return createHash("sha256")
    .update(readText(manifestPath))
    .update("\0")
    .update(readText(lockfilePath))
    .digest("hex");
}

function directDependenciesInstalled() {
  if (!existsSync(nodeModulesPath)) {
    return false;
  }

  const manifest = readManifest();
  const packageNames = [
    ...Object.keys(manifest.dependencies ?? {}),
    ...Object.keys(manifest.devDependencies ?? {}),
  ];

  return packageNames.every((packageName) =>
    existsSync(join(nodeModulesPath, packageName)),
  );
}

const desiredHash = expectedHash();
const currentHash = existsSync(markerPath) ? readText(markerPath).trim() : "";

if (currentHash === desiredHash && directDependenciesInstalled()) {
  console.log("[web] node_modules already matches package manifests");
  process.exit(0);
}

console.log("[web] syncing node_modules with package manifests");

const npmCommand = process.platform === "win32" ? "npm.cmd" : "npm";
const install = spawnSync(npmCommand, ["ci"], {
  cwd: projectRoot,
  env: process.env,
  stdio: "inherit",
});

if (install.status !== 0) {
  process.exit(install.status ?? 1);
}

mkdirSync(nodeModulesPath, { recursive: true });
writeFileSync(markerPath, `${desiredHash}\n`);

console.log("[web] node_modules synced");