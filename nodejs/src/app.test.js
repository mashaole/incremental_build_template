import assert from "node:assert/strict";
import http from "node:http";
import { describe, it } from "node:test";
import { createLimiter } from "./rate-limit.js";
import { createApp, httpBaseURL } from "./app.js";

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

  it("GET /call-golang returns the golang hello body", async () => {
    const go = http.createServer((_req, res) => {
      res.setHeader("content-type", "text/plain");
      res.end("hello world-golangs\n");
    });
    await new Promise((resolve) => go.listen(0, "127.0.0.1", resolve));
    const goURL = `http://127.0.0.1:${go.address().port}`;
    const app = createApp({
      limiter: createLimiter({ rps: 20, burst: 20, maxKeys: 100 }),
      golangURL: goURL,
    });
    const { server, url } = await listen(app);
    try {
      const res = await fetch(url + "/call-golang");
      assert.equal(res.status, 200);
      assert.equal(await res.text(), "hello world-golangs\n");
    } finally {
      server.close();
      go.close();
    }
  });

  it("GET /call-golang is 502 when golang is down", async () => {
    const app = createApp({
      limiter: createLimiter({ rps: 20, burst: 20, maxKeys: 100 }),
      golangURL: "http://127.0.0.1:1",
    });
    const { server, url } = await listen(app);
    try {
      const res = await fetch(url + "/call-golang");
      assert.equal(res.status, 502);
    } finally {
      server.close();
    }
  });
});

describe("httpBaseURL", () => {
  it("defaults to localhost golang", () => {
    assert.equal(httpBaseURL(""), "http://127.0.0.1:8080");
  });

  it("strips a trailing slash", () => {
    assert.equal(
      httpBaseURL("https://golang-api.example.run.app/"),
      "https://golang-api.example.run.app",
    );
  });

  it("rejects non-http schemes", () => {
    assert.throws(() => httpBaseURL("ftp://evil"), /http or https/);
  });
});
