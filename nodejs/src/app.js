import express from "express";
import helmet from "helmet";
import { limiterFromEnv, rateLimit } from "./rate-limit.js";

const HELLO = "hello world-nodes\n";
const DEFAULT_GOLANG_URL = "http://127.0.0.1:8080";

export function createApp({ limiter, golangURL } = {}) {
  const app = express();
  const lim = limiter === undefined ? limiterFromEnv() : limiter;
  const goURL = httpBaseURL(golangURL ?? process.env.GOLANG_URL, DEFAULT_GOLANG_URL);

  app.disable("x-powered-by");
  app.set("trust proxy", 1);
  app.use(helmet());
  app.use(express.json({ limit: "32kb" }));
  app.use(requestLog);
  app.use(rateLimit(lim));

  app.get("/", (_req, res) => {
    res.type("text/plain").send(HELLO);
  });

  app.post("/", (_req, res) => {
    res.type("text/plain").send(HELLO);
  });

  app.get("/call-golang", async (_req, res) => {
    try {
      const ac = new AbortController();
      const timer = setTimeout(() => ac.abort(), 3000);
      const r = await fetch(`${goURL}/`, { signal: ac.signal });
      clearTimeout(timer);
      const body = await r.text();
      if (!r.ok || body.length > 4096) {
        res.status(502).type("text/plain").send("golang unavailable\n");
        return;
      }
      res.type("text/plain").send(body);
    } catch {
      res.status(502).type("text/plain").send("golang unavailable\n");
    }
  });

  app.use((_req, res) => {
    res.status(404).type("text/plain").send("not found\n");
  });

  app.use((err, _req, res, _next) => {
    console.error(
      JSON.stringify({
        msg: "unhandled_error",
        error: err instanceof Error ? err.message : "unknown",
      }),
    );
    res.status(500).type("text/plain").send("internal server error\n");
  });

  return app;
}

export function httpBaseURL(raw, fallback = DEFAULT_GOLANG_URL) {
  const value = (raw ?? "").trim() || fallback;
  let parsed;
  try {
    parsed = new URL(value);
  } catch {
    throw new Error("GOLANG_URL must be a valid URL");
  }
  if (parsed.protocol !== "http:" && parsed.protocol !== "https:") {
    throw new Error("GOLANG_URL must be http or https");
  }
  if (!parsed.hostname) {
    throw new Error("GOLANG_URL host required");
  }
  parsed.hash = "";
  return parsed.toString().replace(/\/$/, "");
}

function requestLog(req, res, next) {
  const start = Date.now();
  res.on("finish", () => {
    console.log(
      JSON.stringify({
        msg: "request",
        method: req.method,
        path: req.path,
        status: res.statusCode,
        duration_ms: Date.now() - start,
      }),
    );
  });
  next();
}
