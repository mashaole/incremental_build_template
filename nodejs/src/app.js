import express from "express";
import helmet from "helmet";
import { limiterFromEnv, rateLimit } from "./rate-limit.js";

const HELLO = "hello world-nodes\n";

export function createApp({ limiter } = {}) {
  const app = express();
  const lim = limiter === undefined ? limiterFromEnv() : limiter;

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
