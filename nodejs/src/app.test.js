import assert from "node:assert/strict";
import { describe, it } from "node:test";
import { createLimiter } from "./rate-limit.js";
import { createApp } from "./app.js";

function listen(app) {
  return new Promise((resolve) => {
    const server = app.listen(0, "127.0.0.1", () => {
      const { port } = server.address();
      resolve({
        server,
        url: `http://127.0.0.1:${port}`,
      });
    });
  });
}

describe("nodejs-api routes", () => {
  it("returns hello world on GET /", async () => {
    const app = createApp({ limiter: createLimiter({ rps: 20, burst: 20, maxKeys: 100 }) });
    const { server, url } = await listen(app);
    try {
      const res = await fetch(url + "/");
      assert.equal(res.status, 200);
      assert.equal(await res.text(), "hello world-nodes\n");
    } finally {
      server.close();
    }
  });

  it("returns hello world on POST /", async () => {
    const app = createApp({ limiter: createLimiter({ rps: 20, burst: 20, maxKeys: 100 }) });
    const { server, url } = await listen(app);
    try {
      const res = await fetch(url + "/", { method: "POST" });
      assert.equal(res.status, 200);
      assert.equal(await res.text(), "hello world-nodes\n");
    } finally {
      server.close();
    }
  });

  it("returns 404 for unknown paths", async () => {
    const app = createApp({
      limiter: createLimiter({ rps: 20, burst: 20, maxKeys: 100 }),
    });
    const { server, url } = await listen(app);
    try {
      const res = await fetch(url + "/nope");
      assert.equal(res.status, 404);
    } finally {
      server.close();
    }
  });

  it("returns 429 after the burst is exhausted", async () => {
    const app = createApp({
      limiter: createLimiter({ rps: 0.001, burst: 1, maxKeys: 100 }),
    });
    const { server, url } = await listen(app);
    try {
      const first = await fetch(url + "/");
      const second = await fetch(url + "/");
      assert.equal(first.status, 200);
      assert.equal(second.status, 429);
      assert.equal(second.headers.get("retry-after"), "1");
    } finally {
      server.close();
    }
  });
});
