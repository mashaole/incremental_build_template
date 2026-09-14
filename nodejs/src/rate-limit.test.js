import assert from "node:assert/strict";
import { describe, it } from "node:test";
import {
  clientIP,
  createLimiter,
  limiterFromEnv,
  rateLimit,
} from "./rate-limit.js";

describe("token bucket", () => {
  it("allows burst then rejects", () => {
    const lim = createLimiter({ rps: 0.001, burst: 2, maxKeys: 8 });
    assert.equal(lim.allow("1.1.1.1"), true);
    assert.equal(lim.allow("1.1.1.1"), true);
    assert.equal(lim.allow("1.1.1.1"), false);
    assert.equal(lim.allow("8.8.8.8"), true);
  });

  it("rejects new keys when full", () => {
    const lim = createLimiter({ rps: 10, burst: 1, maxKeys: 1 });
    assert.equal(lim.allow("1.1.1.1"), true);
    assert.equal(lim.allow("2.2.2.2"), false);
  });
});

describe("clientIP", () => {
  it("uses the first X-Forwarded-For hop", () => {
    const req = {
      headers: { "x-forwarded-for": "203.0.113.10, 192.0.2.1" },
      socket: { remoteAddress: "192.0.2.1" },
    };
    assert.equal(clientIP(req), "203.0.113.10");
  });
});

describe("limiterFromEnv", () => {
  it("disables when RATE_LIMIT_RPS is 0", () => {
    assert.equal(limiterFromEnv({ RATE_LIMIT_RPS: "0" }), null);
  });
});

describe("rateLimit middleware", () => {
  it("sends 429 when the bucket is empty", () => {
    const lim = createLimiter({ rps: 0.001, burst: 1, maxKeys: 8 });
    const mw = rateLimit(lim);
    const req = {
      headers: {},
      socket: { remoteAddress: "203.0.113.9" },
    };
    let status = 0;
    let body = "";
    const res = {
      set() {},
      status(code) {
        status = code;
        return this;
      },
      type() {
        return this;
      },
      send(payload) {
        body = payload;
      },
    };
    let continued = 0;
    mw(req, res, () => {
      continued += 1;
    });
    mw(req, res, () => {
      continued += 1;
    });
    assert.equal(continued, 1);
    assert.equal(status, 429);
    assert.equal(body, "too many requests\n");
  });
});
