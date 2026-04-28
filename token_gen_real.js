const crypto = require("crypto");
const secret = "dev-secret-change-me";
const head = Buffer.from(JSON.stringify({ alg: "HS256", typ: "JWT" })).toString("base64").replace(/=/g, "").replace(/\+/g, "-").replace(/\//g, "_");
const body = Buffer.from(JSON.stringify({
  sub: "cccccccc-cccc-cccc-cccc-ccccccccc005",
  name: "Mike Wade",
  email: "mikewade2k16@gmail.com",
  role: "manager",
  storeIds: ["bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbb002"],
  iat: Math.floor(Date.now() / 1000) - 30,
  exp: Math.floor(Date.now() / 1000) + 3600
})).toString("base64").replace(/=/g, "").replace(/\+/g, "-").replace(/\//g, "_");
const sig = crypto.createHmac("sha256", secret).update(head + "." + body).digest("base64").replace(/=/g, "").replace(/\+/g, "-").replace(/\//g, "_");
process.stdout.write("ldv1." + head + "." + body + "." + sig);
