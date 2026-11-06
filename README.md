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
