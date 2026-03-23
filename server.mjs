import { createReadStream, existsSync, statSync } from "node:fs";
import { extname, join, normalize } from "node:path";
import { createServer } from "node:http";

const host = "127.0.0.1";
const port = Number(process.env.PORT || 4173);
const rootDir = process.cwd();

const mimeTypes = {
  ".css": "text/css; charset=utf-8",
  ".html": "text/html; charset=utf-8",
  ".js": "application/javascript; charset=utf-8",
  ".json": "application/json; charset=utf-8",
  ".svg": "image/svg+xml; charset=utf-8",
  ".txt": "text/plain; charset=utf-8"
};

function resolveFilePath(urlPath) {
  const sanitizedPath = normalize(urlPath).replace(/^(\.\.[/\\])+/, "");
  const requestedPath = sanitizedPath === "/" ? "index.html" : sanitizedPath.slice(1);
  const filePath = join(rootDir, requestedPath);

  if (existsSync(filePath) && statSync(filePath).isDirectory()) {
    return join(filePath, "index.html");
  }

  return filePath;
}

createServer((request, response) => {
  const requestUrl = new URL(request.url || "/", `http://${host}:${port}`);
  const filePath = resolveFilePath(requestUrl.pathname);

  if (!existsSync(filePath)) {
    response.writeHead(404, { "Content-Type": "text/plain; charset=utf-8" });
    response.end("Arquivo nao encontrado.");
    return;
  }

  const contentType = mimeTypes[extname(filePath)] || "application/octet-stream";

  response.writeHead(200, { "Content-Type": contentType });
  createReadStream(filePath).pipe(response);
}).listen(port, host, () => {
  console.log(`MVP disponivel em http://${host}:${port}`);
});
