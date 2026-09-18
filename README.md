# Incremental Cloud Build template

One GitHub repo, two tiny APIs (`golang-api` and `nodejs-api`). A commit that
only touches `golang/` should rebuild and redeploy **only** that service on
Cloud Run. Same for Node. There is **no** `cloudbuild.yaml` and **no** Cloud
Run YAML — Cloud Run + Cloud Build triggers are created in the GCP console.

```text
.
├── golang/            Go API  (Cloud Run: golang-api, local :8080)
├── nodejs/            Node API (Cloud Run: nodejs-api, local :8081)
├── docker-compose.yml local run-together
├── Makefile           setup / start / stop
└── scripts/dev.sh     implementation behind the Makefile
```

## Local

Needs Go 1.24+, Node 20+, and Docker (optional — without Docker, processes
start natively).

```bash
make setup                 # deps + images
make start                 # both services
make start-golang          # Go only
make start-nodejs          # Node only
make status
make logs
make stop
make test
```

You can call `./scripts/dev.sh` with the same commands; `make` is just a
shortcut. The script decides Docker vs native start, tracks native pids, and
stops everything.

| Service | URL |
|---------|-----|
| golang-api | http://127.0.0.1:8080/ |
| nodejs-api | http://127.0.0.1:8081/ |

Both accept `GET /` and `POST /`. Go responds with `hello world-golangs`;
Node responds with `hello world-nodes`.

Per-IP rate limit (in-process token bucket): **5 requests/sec**, burst **20**.
Tune with `RATE_LIMIT_RPS`, `RATE_LIMIT_BURST`, and `RATE_LIMIT_MAX_KEYS`.
Set `RATE_LIMIT_RPS=0` to disable. This is **per Cloud Run instance**; for
project-wide throttling add Cloud Armor in front of the services.

Exceeded clients get `429` and `Retry-After: 1`.

### Calling each other

Each service has one env var: the other service’s base URL. Empty means
localhost.

| Variable | Service | Default if unset |
|----------|---------|------------------|
| `NODEJS_URL` | golang-api | `http://127.0.0.1:8081` |
| `GOLANG_URL` | nodejs-api | `http://127.0.0.1:8080` |

`GET /call-nodejs` on Go does `GET {NODEJS_URL}/` and returns that body.
`GET /call-golang` on Node does `GET {GOLANG_URL}/` and returns that body.

Cloud Run (project `incremental-build-template`):

```text
# golang-api
NODEJS_URL=https://nodejs-api-64192940138.europe-west1.run.app

# nodejs-api
GOLANG_URL=https://golang-api-64192940138.europe-west1.run.app
```

Local:

```bash
curl http://127.0.0.1:8080/call-nodejs    # hello world-nodes
curl http://127.0.0.1:8081/call-golang    # hello world-golangs
```

After deploy (and those env vars set in the Cloud Run UI):

```bash
curl https://golang-api-64192940138.europe-west1.run.app/call-nodejs
curl https://nodejs-api-64192940138.europe-west1.run.app/call-golang
```

If Docker is not running, `make start*` uses native processes and writes
pids/logs under `.run/`. `make stop` tears down containers **and** native
pids.

## GCP UI: connect GitHub → Cloud Build → Cloud Run

Do this once per service. The repo stays yaml-free; incremental behavior
comes from **included files** filters on the Cloud Build triggers.

### 0. One-time project setup

1. In [Google Cloud Console](https://console.cloud.google.com/) pick the project.
2. Enable APIs: **Cloud Run**, **Cloud Build**, **Artifact Registry**.
3. Connect GitHub:
   - **Cloud Build → Repositories → 2nd gen** (Developer Connect), or
   - the Cloud Build GitHub App when Cloud Run asks you to connect a repo.
4. Grant the Cloud Build service account:
   - **Cloud Run Admin**
   - **Service Account User**
   - **Artifact Registry Writer**  
   (the Cloud Run “continuous deploy” wizard usually does this).

### 1. Create each Cloud Run service from source

Repeat for `golang-api` and `nodejs-api`.

1. **Cloud Run → Create service**.
2. Choose **Continuously deploy from a repository** (source, not a
   pre-built image).
3. Select this GitHub repo and the branch (`main` or `master`).
4. Build type: **Dockerfile** (not Buildpacks).
5. Source / Docker context:
   - Go: `golang`
   - Node: `nodejs`
6. Dockerfile name: `Dockerfile`.
7. Service name: `golang-api` or `nodejs-api`.
8. Region, CPU, memory — your choice. Container port **8080**.
9. Create the service. Cloud Run stores the build config **on the trigger**,
   not in this repo.

Cloud Run sets `PORT=8080`. Both images already listen on `$PORT`.

### 2. Make builds incremental (this is the important part)

The wizard creates a trigger that fires on **every** push. Restrict it:

1. **Cloud Build → Triggers**.
2. Open the trigger Cloud Run created for `golang-api`.
3. **Edit → Included files** (sometimes under “Show included files filter”):

   ```text
   golang/**
   ```

4. Save.
5. Repeat for `nodejs-api`:

   ```text
   nodejs/**
   ```

Optional ignored files (either trigger): `**/*.md`, `**/*_test.go`,
`**/*.test.js`.

After this:

| Commit touches | What Cloud Build runs |
|----------------|------------------------|
| `golang/**` only | golang-api image + Cloud Run deploy |
| `nodejs/**` only | nodejs-api image + Cloud Run deploy |
| both | both triggers |
| README / Makefile only | nothing |

### 3. Verify

1. Change a string in `golang/internal/httpserver/server.go`.
2. Push to the connected branch.
3. **Cloud Build → History**: only the golang trigger should run.
4. Cloud Run `golang-api` revision should update; `nodejs-api` should not.

## Notes

- Do not add a repo-root `cloudbuild.yaml` unless you later want in-repo
  pipelines. UI-managed Dockerfile triggers are enough.
- If you later add a shared folder (for example `libs/`), add that glob to
  **both** included-files filters.
- Local compose maps Node to **8081** so both can run together. Cloud Run
  still uses **8080** inside each container.
