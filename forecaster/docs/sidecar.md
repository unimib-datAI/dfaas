# Sidecar model promotion (low-resource)

This sidecar is designed for k3s edge deployments with minimal CPU/RAM usage.
It promotes a staged model bundle into `/models/current` and optionally triggers
`/reload`.

## Requirements
- Shared volume mounted at `/models`
- `jq` installed if you want to read `sidecar.poll_interval_seconds`

## Flow
1) An external process places a model bundle in `/models/staging`
2) It creates `/models/staging/READY`
3) The sidecar atomically promotes the bundle and updates `/models/current`
4) Optionally triggers `/reload`

## Configuration (env)
- `MODELS_ROOT` (default `/models`)
- `STAGING_DIR` (default `/models/staging`)
- `CURRENT_LINK` (default `/models/current`)
- `MANIFEST_NAME` (default `manifest.json`)
- `POLL_DEFAULT` (default `30`)
- `RELOAD_URL` (default `http://127.0.0.1:8000/reload`)
- `RELOAD_TOKEN` (optional)

## CLI option
You can override polling directly:
```bash
./sidecar/sidecar.sh --poll-interval 10
```

## Manifest-driven polling
If the manifest contains:
```json
{
  "sidecar": { "poll_interval_seconds": 30 }
}
```
and `jq` is available, the sidecar uses that interval; otherwise it falls back
to `POLL_DEFAULT`.

## Run (host)
```bash
./sidecar/sidecar.sh
```

## Build image
```bash
container build -t forecaster-sidecar:latest -f sidecar/Dockerfile sidecar
```

## Run with apple/container
```bash
container run --rm -it \
  -v /var/lib/forecaster-models:/models \
  forecaster-sidecar:latest --poll-interval 10
```

## Full k3s E2E
See `tests/test_e2e_k3s.py` and run:
```bash
uv run pytest -m e2e_k3s
```

## k3s manifest
Apply:
```bash
kubectl apply -f k8s/forecaster-sidecar.yaml
```

## Minimal k3s snippet
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: forecaster
spec:
  replicas: 1
  selector:
    matchLabels:
      app: forecaster
  template:
    metadata:
      labels:
        app: forecaster
    spec:
      containers:
        - name: app
          image: forecaster:latest
          ports:
            - containerPort: 8000
          volumeMounts:
            - name: models
              mountPath: /models
        - name: sidecar
          image: forecaster-sidecar:latest
          command: ["/sidecar/sidecar.sh"]
          volumeMounts:
            - name: models
              mountPath: /models
      volumes:
        - name: models
          hostPath:
            path: /var/lib/forecaster-models
            type: DirectoryOrCreate
```
