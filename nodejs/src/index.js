import { createApp } from "./app.js";

const PORT = process.env.PORT || "8080";
const app = createApp();

const server = app.listen(PORT, "0.0.0.0", () => {
  console.log(
    JSON.stringify({
      msg: "nodejs-api listening",
      port: PORT,
    }),
  );
});

server.requestTimeout = 10_000;
server.headersTimeout = 11_000;

function shutdown(signal) {
  console.log(JSON.stringify({ msg: "shutdown", signal }));
  server.close((err) => {
    if (err) {
      console.error(JSON.stringify({ msg: "shutdown_error", error: err.message }));
      process.exit(1);
    }
    process.exit(0);
  });
  setTimeout(() => process.exit(1), 10_000).unref();
}

process.on("SIGTERM", () => shutdown("SIGTERM"));
process.on("SIGINT", () => shutdown("SIGINT"));
