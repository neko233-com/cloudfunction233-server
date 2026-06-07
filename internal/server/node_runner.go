package server

const nodeRunnerSource = `
import { Buffer } from "node:buffer";

const [entrypoint = "index.js", handlerName = "fetch"] = process.argv.slice(2);
const chunks = [];
for await (const chunk of process.stdin) chunks.push(chunk);
const payload = JSON.parse(Buffer.concat(chunks).toString("utf8") || "{}");

const mod = await import(new URL(entrypoint, import.meta.url).href);
const target = resolveHandler(mod, handlerName);
if (typeof target !== "function") {
  throw new Error("handler " + handlerName + " was not found");
}

const request = toRequest(payload);
const env = payload.env || {};
const ctx = {
  waitUntil(promise) {
    Promise.resolve(promise).catch((err) => console.error(err));
  },
  remainingPath: payload.remainingPath || "/"
};

const result = await target(request, env, ctx);
const response = await normalizeResponse(result);
process.stdout.write(JSON.stringify(response));

function resolveHandler(mod, name) {
  if (name.includes(".")) {
    return name.split(".").reduce((value, key) => value?.[key], mod);
  }
  return mod[name] || mod.default?.[name] || mod.default;
}

function toRequest(payload) {
  const headers = new Headers();
  for (const [key, values] of Object.entries(payload.headers || {})) {
    for (const value of values || []) headers.append(key, value);
  }
  const init = { method: payload.method || "GET", headers };
  if (!["GET", "HEAD"].includes(init.method.toUpperCase()) && payload.body) {
    init.body = payload.base64 ? Buffer.from(payload.body, "base64") : payload.body;
  }
  const request = new Request(payload.url || "http://localhost/", init);
  request.cf233 = {
    path: payload.path || "/",
    remainingPath: payload.remainingPath || "/",
    query: payload.query || ""
  };
  return request;
}

async function normalizeResponse(value) {
  if (value instanceof Response) {
    const body = Buffer.from(await value.arrayBuffer());
    return {
      status: value.status,
      headers: Object.fromEntries(value.headers.entries()),
      body: body.toString("base64"),
      base64: true
    };
  }
  if (typeof value === "string" || value instanceof String) {
    return { status: 200, headers: { "content-type": "text/plain; charset=utf-8" }, body: String(value), base64: false };
  }
  if (value && typeof value === "object" && ("status" in value || "body" in value || "headers" in value)) {
    return {
      status: value.status || 200,
      headers: value.headers || { "content-type": "application/json; charset=utf-8" },
      body: value.base64 ? value.body || "" : String(value.body ?? ""),
      base64: Boolean(value.base64)
    };
  }
  return { status: 200, headers: { "content-type": "application/json; charset=utf-8" }, body: JSON.stringify(value ?? null), base64: false };
}
`
