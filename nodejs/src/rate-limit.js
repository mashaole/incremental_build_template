import { isIP } from "node:net";

// Per-IP token bucket. Allow is O(1) average; space is O(min(keys, maxKeys)).

const DEFAULT_RPS = 5;
const DEFAULT_BURST = 20;
const DEFAULT_MAX_KEYS = 10_000;

export function limiterFromEnv(env = process.env) {
  const rps = parseBoundFloat(env.RATE_LIMIT_RPS, DEFAULT_RPS);
  if (rps <= 0) {
    return null;
  }
  return createLimiter({
    rps,
    burst: parseBoundInt(env.RATE_LIMIT_BURST, DEFAULT_BURST),
    maxKeys: parseBoundInt(env.RATE_LIMIT_MAX_KEYS, DEFAULT_MAX_KEYS),
  });
}

export function createLimiter({ rps, burst, maxKeys }) {
  const rate = rps > 0 ? rps : DEFAULT_RPS;
  const cap = burst >= 1 ? burst : DEFAULT_BURST;
  const limit = maxKeys >= 1 ? maxKeys : DEFAULT_MAX_KEYS;
  const buckets = new Map();

  return {
    allow(key) {
      if (!key) {
        return true;
      }
      const now = Date.now();
      const b = buckets.get(key);
      if (!b) {
        if (buckets.size >= limit) {
          return false;
        }
        buckets.set(key, { tokens: cap - 1, last: now });
        return true;
      }
      const elapsed = Math.max(0, (now - b.last) / 1000);
      b.tokens = Math.min(cap, b.tokens + elapsed * rate);
      b.last = now;
      if (b.tokens < 1) {
        return false;
      }
      b.tokens -= 1;
      return true;
    },
    size() {
      return buckets.size;
    },
  };
}

export function rateLimit(limiter) {
  if (!limiter) {
    return (_req, _res, next) => next();
  }
  return (req, res, next) => {
    if (!limiter.allow(clientIP(req))) {
      res.set("Retry-After", "1");
      res.status(429).type("text/plain").send("too many requests\n");
      return;
    }
    next();
  };
}

export function clientIP(req) {
  const xff = req.headers["x-forwarded-for"];
  if (typeof xff === "string" && xff.length > 0) {
    const first = xff.split(",", 2)[0].trim();
    if (isIP(first)) {
      return first;
    }
  }
  const raw = req.socket?.remoteAddress || "";
  return raw.replace(/^::ffff:/, "") || "unknown";
}

function parseBoundFloat(raw, fallback) {
  if (raw === undefined || raw === "") {
    return fallback;
  }
  const n = Number(raw);
  if (!Number.isFinite(n) || n < 0) {
    return fallback;
  }
  return n;
}

function parseBoundInt(raw, fallback) {
  if (raw === undefined || raw === "") {
    return fallback;
  }
  const n = Number.parseInt(raw, 10);
  if (!Number.isFinite(n) || n < 0) {
    return fallback;
  }
  return n;
}
