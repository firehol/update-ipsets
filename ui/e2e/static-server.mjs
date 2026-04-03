import { createServer } from "node:http";
import { createReadStream, existsSync, statSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const here = path.dirname(fileURLToPath(import.meta.url));
const uiRoot = path.resolve(here, "..");
const distRoot = path.join(uiRoot, "dist");
const port = readPort(process.argv);

const server = createServer((request, response) => {
  if (!request.url) {
    response.writeHead(400);
    response.end("missing url");
    return;
  }

  const url = new URL(request.url, `http://${request.headers.host ?? "127.0.0.1"}`);
  if (request.method !== "GET" && request.method !== "HEAD") {
    response.writeHead(405);
    response.end("method not allowed");
    return;
  }

  const assetPath = assetPathForURL(url.pathname);
  if (assetPath) {
    serveFile(assetPath, response, request.method === "HEAD");
    return;
  }

  serveFile(path.join(distRoot, "index.html"), response, request.method === "HEAD");
});

server.listen(port, "127.0.0.1", () => {
  console.error(`e2e static server listening on http://127.0.0.1:${port}`);
});

function readPort(argv) {
  const idx = argv.indexOf("--port");
  if (idx >= 0 && argv[idx + 1]) {
    return Number(argv[idx + 1]);
  }
  return Number(process.env.PORT ?? 4173);
}

function assetPathForURL(pathname) {
  const normalized = decodeURIComponent(pathname);
  if (normalized.startsWith("/static/")) {
    return safeJoin(distRoot, normalized.slice("/static/".length));
  }
  if (normalized === "/favicon.svg") {
    return safeJoin(distRoot, "favicon.svg");
  }
  return null;
}

function safeJoin(root, rel) {
  const target = path.resolve(root, rel);
  if (target === root || target.startsWith(`${root}${path.sep}`)) {
    return target;
  }
  return null;
}

function serveFile(filePath, response, headOnly) {
  if (!filePath || !existsSync(filePath) || !statSync(filePath).isFile()) {
    response.writeHead(404);
    response.end("not found");
    return;
  }

  response.writeHead(200, {
    "content-type": contentType(filePath),
    "cache-control": "no-store",
  });
  if (headOnly) {
    response.end();
    return;
  }
  createReadStream(filePath).pipe(response);
}

function contentType(filePath) {
  switch (path.extname(filePath)) {
    case ".css":
      return "text/css; charset=utf-8";
    case ".html":
      return "text/html; charset=utf-8";
    case ".js":
      return "text/javascript; charset=utf-8";
    case ".json":
      return "application/json; charset=utf-8";
    case ".svg":
      return "image/svg+xml";
    case ".woff2":
      return "font/woff2";
    default:
      return "application/octet-stream";
  }
}
