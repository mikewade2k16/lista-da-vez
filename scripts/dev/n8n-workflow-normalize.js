"use strict";

// args: <rawPath> <targetPath> <mode> <expectedId> <module>
// mode: write | check
// stdout: STATUS:written | STATUS:unchanged | STATUS:drift
// exit: 0 ok | 3 credential leak | 4 parse/IO/ID | 5 usage | 6 direct channel

const fs = require("fs");
const ALLOWED_CREDENTIAL_FIELDS = new Set(["id", "name"]);
const DIRECT_CHANNEL_NODE_TYPE = /waha|evolution|whatsapp|instagram|facebook.?graph|meta.?messag/i;
const DIRECT_CHANNEL_URL = /https?:[^\s"']*(?:waha|evolution|graph\.facebook\.com|graph\.instagram\.com|api\.instagram\.com|api\.whatsapp\.com|wa\.me)/i;

function fail(code, message) {
  process.stderr.write(message + "\n");
  process.exit(code);
}

const rawPath = process.argv[2];
const targetPath = process.argv[3];
const mode = process.argv[4];
const expectedId = process.argv[5];
const moduleName = process.argv[6];

if (!rawPath || !targetPath || !expectedId || !moduleName || !["write", "check"].includes(mode)) {
  fail(5, "uso: node n8n-workflow-normalize.js <raw> <target> <write|check> <expectedId> <module>");
}

let parsed;
try {
  parsed = JSON.parse(fs.readFileSync(rawPath, "utf8"));
} catch (_error) {
  fail(4, "export do n8n ilegivel ou invalido.");
}

if (Array.isArray(parsed) && parsed.length !== 1) {
  fail(4, "export do n8n deve conter exatamente um workflow.");
}
const workflow = Array.isArray(parsed) ? parsed[0] : parsed;
if (!workflow || typeof workflow !== "object" || Array.isArray(workflow)) {
  fail(4, "export do n8n vazio ou com shape invalido.");
}
if (String(workflow.id || "") !== expectedId) {
  fail(4, "ID runtime divergente do registro canonico.");
}

for (const node of workflow.nodes || []) {
  const credentials = node && node.credentials;
  if (credentials !== undefined && credentials !== null) {
    if (typeof credentials !== "object" || Array.isArray(credentials)) {
      fail(3, "credential com shape invalido; export abortado.");
    }
    for (const credentialType of Object.keys(credentials)) {
      const credential = credentials[credentialType];
      if (!credential || typeof credential !== "object" || Array.isArray(credential)) {
        fail(3, "credential materializada ou com shape invalido; export abortado.");
      }
      for (const field of Object.keys(credential)) {
        if (!ALLOWED_CREDENTIAL_FIELDS.has(field)) {
          fail(3, "credential contem campo fora de id/name; export abortado.");
        }
      }
    }
  }

  if (moduleName === "omnichannel") {
    const type = String((node && node.type) || "");
    if (DIRECT_CHANNEL_NODE_TYPE.test(type)) {
      fail(6, "node de canal direto proibido no workflow omnichannel; use Go/outbox.");
    }
    const parameters = JSON.stringify((node && node.parameters) || {});
    if (DIRECT_CHANNEL_URL.test(parameters)) {
      fail(6, "endpoint de canal direto proibido no workflow omnichannel; use Go/outbox.");
    }
  }
}

const keys = [
  "id",
  "name",
  "nodes",
  "connections",
  "active",
  "settings",
  "staticData",
  "meta",
  "pinData",
  "versionId",
];
const projected = {};
for (const key of keys) {
  if (key === "active") projected.active = false;
  else if (key === "pinData") projected.pinData = {};
  else if (key === "staticData") projected.staticData = null;
  else if (Object.prototype.hasOwnProperty.call(workflow, key)) projected[key] = workflow[key];
}

const output = JSON.stringify([projected], null, 2) + "\n";
let current = null;
if (fs.existsSync(targetPath)) {
  try {
    current = fs.readFileSync(targetPath, "utf8");
  } catch (_error) {
    current = null;
  }
}
const unchanged = current !== null && current === output;

if (mode === "check") {
  process.stdout.write(unchanged ? "STATUS:unchanged\n" : "STATUS:drift\n");
  process.exit(0);
}
if (unchanged) {
  process.stdout.write("STATUS:unchanged\n");
  process.exit(0);
}

try {
  fs.writeFileSync(targetPath, output);
} catch (_error) {
  fail(4, "falha ao gravar workflow normalizado.");
}
process.stdout.write("STATUS:written\n");
