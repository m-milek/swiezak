# swiezak

Swiezak checks whether the images of your Docker containers are up to date.

For every container it reports:

- **`rebuild`**: the same tag now points to a newer build.
- **`major` / `minor` / `patch`**: newer versions you can switch to.

Supported registries: 
* Docker Hub
* GitHub Container Registry

Images are checked periodically in the background. Results are saved to a JSON file and served from it.

## Configuration
All configuration is optional.

| Variable | Default | Description |
|---|---|---|
| `REFRESH_INTERVAL` | `8h` | How often to check, as a Go duration (`30m`, `1h30m`, `24h`). |
| `REPORT_PATH` | `/data/report.json` | Where the result is stored. |
| `DOCKERHUB_USERNAME`, `DOCKERHUB_TOKEN` | | Docker Hub login, raises the anonymous rate limit. |
| `GHCR_USERNAME`, `GHCR_TOKEN` | | GitHub Container Registry login. |

## API

`GET /api/v1/docker` returns one entry per container image (`503` until the first check finishes).

```json
{
  "image": "traefik/whoami:v1.10",
  "pinned": false,
  "status": "up-to-date",
  "major": [],
  "minor": ["v1.12", "v1.11"],
  "patch": [],
  "checkedAt": "2026-11-06T12:00:00Z"
}
```

`status` and the version lists are independent:

- `status` describes the **current tag**: `up-to-date`, `rebuild` (the tag points to a newer build), `pinned` (digest only, no tag, so nothing to compare), `not-checkable` (local image or unknown to the registry), `error`.
- `major` / `minor` / `patch` list **newer tags**, newest first. An `up-to-date` image can still have newer versions.
- `pinned` is `true` whenever the image has a digest. With a tag as well (`alpine:3.20@sha256:…`) it is still checked normally; only a bare digest gets `status: "pinned"`.
- `error` holds the error message when `status` is `error`.
- `checkedAt` is when the entry was checked (UTC, RFC 3339).
