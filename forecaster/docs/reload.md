# Model reload design

This service supports a robust, k8s-friendly model reload flow based on a
versioned model directory and a manifest with checksums.

## Layout
The model base directory contains `models/` and `scalers/` plus a manifest:
```
/models/current
  manifest.json
  models/
    cpu_usage_node/regression/model.joblib
    ram_usage_node/regression/model.joblib
    power_usage_node/regression/model.joblib
  scalers/
    scaler_x/features.joblib
    scaler_y/cpu_usage_node.joblib
    scaler_y/ram_usage_node.joblib
    scaler_y/power_usage_node.joblib
```

For versioning, create versioned folders and repoint `/models/current`:
```
/models/v2026-01-30T0900Z/...
/models/current -> /models/v2026-01-30T0900Z
```

## Manifest
`manifest.json` lives under the base directory and includes a checksum for each
file that must be loaded.

Example:
```json
{
  "model_version": "v2026-01-30T0900Z",
  "models_type": "regression",
  "created_at": "2026-01-30T09:00:00Z",
  "sha256": {
    "models/cpu_usage_node/regression/model.joblib": "<hash>",
    "models/ram_usage_node/regression/model.joblib": "<hash>",
    "models/power_usage_node/regression/model.joblib": "<hash>",
    "scalers/scaler_x/features.joblib": "<hash>",
    "scalers/scaler_y/cpu_usage_node.joblib": "<hash>",
    "scalers/scaler_y/ram_usage_node.joblib": "<hash>",
    "scalers/scaler_y/power_usage_node.joblib": "<hash>"
  }
}
```

## Generate manifest
You can generate a manifest for the current bundle with:
```
uv run python scripts/generate_manifest.py --models-type regression --base-dir .
```

Use `--dry-run` to print the manifest without writing a file.

Shell helper (no Python required):
```
MODELS_TYPE=regression ./scripts/generate_manifest.sh
```

### Optional sidecar configuration
You can add a `sidecar` section to control poll interval:
```json
{
  "sidecar": {
    "poll_interval_seconds": 30
  }
}
```

## Environment variables
Defaults come from `config/runtime.json`. Environment variables are documented
below but are not used unless env overrides are explicitly enabled in code.

- `MODELS_BASE_DIR` (default: `/models/current`)
- `MODELS_TYPE` (required, must match manifest `models_type`)
- `MODELS_MANIFEST` (default: `manifest.json`)
- `RELOAD_MODE` (`none` | `poll` | `endpoint`)
- `RELOAD_INTERVAL_SECONDS` (default: `30`, used when polling)
- `RELOAD_TOKEN` (optional, required for `/reload` when set)

For local development with the current repo layout, set:
```
MODELS_BASE_DIR=.
```

## Reload modes
### Polling
Set `RELOAD_MODE=poll` and update the manifest on disk. The service checks the
manifest fingerprint every `RELOAD_INTERVAL_SECONDS`.

### Endpoint
Set `RELOAD_MODE=endpoint` and call:
```
POST /reload
```
If `RELOAD_TOKEN` is set, include the header `x-reload-token`.

## Readiness
- `/health` includes model version and last error
- `/ready` returns `ready=true` only when all models are loaded

## k3s compatibility
This design works with a directory mount on the node (hostPath/CSI). You can
update the folder out-of-band (sidecar or DaemonSet) and use the reload modes
above to activate the new models.

For a low-resource sidecar example, see `docs/sidecar.md`.

## E2E testing with apple/container
When running E2E tests locally, ensure the container system is started:
```
container system start
```
