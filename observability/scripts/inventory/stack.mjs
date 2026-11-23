// Stack tecnologica: Go, Nuxt, Docker Compose, CI e scripts de deploy.
import path from "node:path";
import {
  ROOT,
  readText,
  readJson,
  exists,
  walk,
  rel,
  uniqSorted,
} from "./util.mjs";

function goStack() {
  const src = readText(path.join(ROOT, "back/go.mod"));
  const direct = [];
  for (const m of src.matchAll(
    /^(?:require\s+|[ \t]+)([\w./-]+)\s+(v[\w.+-]+)(.*)$/gm,
  )) {
    if (!/\/\/ indirect/.test(m[3])) direct.push(`${m[1]}@${m[2]}`);
  }
  return {
    language: `go ${(src.match(/^go\s+([\d.]+)/m) || [])[1] || "?"}`,
    module: (src.match(/^module\s+(\S+)/m) || [])[1],
    directDependencies: uniqSorted(direct),
  };
}

function webStack() {
  const pkg = readJson(path.join(ROOT, "web/package.json"));
  const deps = { ...pkg.dependencies, ...pkg.devDependencies };
  const pick = (names) =>
    Object.fromEntries(names.filter((n) => deps[n]).map((n) => [n, deps[n]]));
  return {
    framework: pick([
      "nuxt",
      "vue",
      "vue-router",
      "pinia",
      "tailwindcss",
      "@tailwindcss/vite",
      "@nuxtjs/tailwindcss",
      "typescript",
      "vitest",
      "@playwright/test",
    ]),
    scripts: Object.keys(pkg.scripts || {}).sort(),
  };
}

/** Parse leve de docker-compose (sem lib YAML): servicos, imagem/build, profiles, portas. */
function composeServices(file) {
  const lines = readText(file).split("\n");
  const services = [];
  let inServices = false;
  let current = null;
  let section = null;
  for (const line of lines) {
    if (/^services:\s*$/.test(line)) {
      inServices = true;
      continue;
    }
    if (inServices && /^\S/.test(line) && !/^\s*#/.test(line))
      inServices = false;
    if (!inServices) continue;
    const svc = line.match(/^ {2}([\w-]+):\s*$/);
    if (svc) {
      current = {
        name: svc[1],
        image: null,
        build: false,
        profiles: [],
        ports: [],
      };
      services.push(current);
      section = null;
      continue;
    }
    if (!current) continue;
    const key = line.match(/^ {4}([\w_]+):\s*(.*)$/);
    if (key) {
      section = key[1];
      if (key[1] === "image")
        current.image = key[2]
          .replace(/\s+#.*$/, "")
          .replace(/['"]/g, "")
          .trim();
      if (key[1] === "build") current.build = true;
      if (key[1] === "profiles")
        current.profiles = [...key[2].matchAll(/[\w-]+/g)].map((m) => m[0]);
      continue;
    }
    const item = line.match(/^ {6}-\s*["']?([^"'#]+?)["']?\s*(?:#.*)?$/);
    if (item && section === "ports") current.ports.push(item[1]);
    if (item && section === "profiles") current.profiles.push(item[1]);
  }
  return services.sort((a, b) => a.name.localeCompare(b.name));
}

function workflows() {
  const dir = path.join(ROOT, ".github/workflows");
  return walk(dir, (f) => /\.ya?ml$/.test(f)).map((abs) => {
    const src = readText(abs);
    const jobsBlock = src.split(/^jobs:\s*$/m)[1] || "";
    return {
      file: rel(abs),
      name:
        (src.match(/^name:\s*(.+)$/m) || [])[1]?.trim() || path.basename(abs),
      jobs: uniqSorted(
        [...jobsBlock.matchAll(/^ {2}([\w-]+):\s*$/gm)].map((m) => m[1]),
      ),
    };
  });
}

export function readStack() {
  const rootPkg = readJson(path.join(ROOT, "package.json"));
  const compose = [
    "docker-compose.yml",
    "docker-compose.prod.yml",
    "compose.mac-local.yaml",
  ]
    .filter((f) => exists(path.join(ROOT, f)))
    .map((f) => ({ file: f, services: composeServices(path.join(ROOT, f)) }));
  const names = new Set(compose.flatMap((c) => c.services.map((s) => s.name)));
  return {
    backend: goStack(),
    frontend: webStack(),
    database: {
      engine: "postgres",
      image:
        compose[0]?.services.find((s) => s.name === "postgres")?.image || null,
      migrations: "back/internal/platform/database/migrations",
    },
    infra: {
      compose,
      redis: names.has("redis"),
      n8n: names.has("n8n"),
      waha: names.has("waha"),
      evolution: names.has("evolution"),
      whisper: names.has("whisper"),
      ciWorkflows: workflows(),
    },
    deploy: Object.fromEntries(
      Object.entries(rootPkg.scripts || {})
        .filter(([k]) => k.startsWith("deploy"))
        .sort(([a], [b]) => a.localeCompare(b)),
    ),
  };
}
