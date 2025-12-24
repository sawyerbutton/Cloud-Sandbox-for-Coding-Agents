# Cloud Sandbox User Guide

## Overview

Cloud Sandbox provides isolated execution environments for AI coding agents. It enables secure code execution, file operations, and session management in containerized sandboxes.

## Quick Start

### Prerequisites

- Docker installed and running
- Go 1.22+ (for building from source)
- Python 3.9+ (for using the Python SDK)

### Running Locally

1. **Start the services:**

```bash
# Start all services
make run-all

# Or start individually:
go run ./cmd/scheduler &
go run ./cmd/session-manager &
go run ./cmd/gateway
```

2. **Verify services are running:**

```bash
curl http://localhost:8080/health
# Expected: {"service":"gateway","status":"ok"}
```

### Using the Python SDK

1. **Install the SDK:**

```bash
pip install -e sdk/python
```

2. **Basic usage:**

```python
from cloud_sandbox import Sandbox

# Create a sandbox with automatic resource management
with Sandbox.create(
    base_url="http://localhost:8080",
    user_id="my-user-id",
) as sandbox:
    # Execute Python code
    result = sandbox.run_code("print('Hello, World!')")
    print(result.stdout)  # Output: Hello, World!

    # Run shell commands
    result = sandbox.run_command("ls -la /workspace")
    print(result.stdout)

    # File operations
    sandbox.write_file("/workspace/test.py", "x = 42")
    files = sandbox.list_files("/workspace")
```

## API Reference

### Authentication

All API endpoints (except `/health` and `/api/v1/auth/token`) require JWT authentication.

**Get Token:**

```bash
curl -X POST http://localhost:8080/api/v1/auth/token \
  -H "Content-Type: application/json" \
  -d '{"user_id": "my-user", "role": "user"}'
```

Response:
```json
{
  "access_token": "eyJhbGci...",
  "refresh_token": "eyJhbGci...",
  "token_type": "Bearer",
  "expires_in": "86400"
}
```

### Sessions

**Create Session:**

```bash
curl -X POST http://localhost:8080/api/v1/sessions \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "image": "python:3.11-slim",
    "cpu_count": 2,
    "memory_mb": 2048
  }'
```

**List Sessions:**

```bash
curl http://localhost:8080/api/v1/sessions \
  -H "Authorization: Bearer $TOKEN"
```

**Pause Session:**

```bash
curl -X POST http://localhost:8080/api/v1/sessions/{session_id}/pause \
  -H "Authorization: Bearer $TOKEN"
```

**Resume Session:**

```bash
curl -X POST http://localhost:8080/api/v1/sessions/{session_id}/resume \
  -H "Authorization: Bearer $TOKEN"
```

**Delete Session:**

```bash
curl -X DELETE http://localhost:8080/api/v1/sessions/{session_id} \
  -H "Authorization: Bearer $TOKEN"
```

### Sandboxes

**Acquire Sandbox:**

```bash
curl -X POST http://localhost:8080/api/v1/sandbox/acquire \
  -H "Authorization: Bearer $TOKEN"
```

Response:
```json
{
  "sandbox_id": "abc123",
  "container_id": "...",
  "status": "active",
  "ip": "172.17.0.5"
}
```

**Release Sandbox:**

```bash
curl -X POST http://localhost:8080/api/v1/sandbox/release \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"sandbox_id": "abc123"}'
```

**Get Pool Stats:**

```bash
curl http://localhost:8080/api/v1/sandbox/stats \
  -H "Authorization: Bearer $TOKEN"
```

### Code Execution

**Execute Code:**

```bash
curl -X POST http://localhost:8080/api/v1/execute \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "sandbox_id": "abc123",
    "code": "print(\"Hello!\")",
    "language": "python",
    "timeout": 30
  }'
```

**Execute Command:**

```bash
curl -X POST http://localhost:8080/api/v1/execute \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "sandbox_id": "abc123",
    "command": ["ls", "-la", "/workspace"]
  }'
```

Response:
```json
{
  "exit_code": 0,
  "stdout": "...",
  "stderr": "",
  "duration_ms": 150,
  "timed_out": false
}
```

### File Operations

**Write File:**

```bash
curl -X PUT http://localhost:8080/api/v1/files \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "sandbox_id": "abc123",
    "path": "/workspace/test.py",
    "content": "print(\"test\")"
  }'
```

**List Files:**

```bash
curl "http://localhost:8080/api/v1/files?sandbox_id=abc123&path=/workspace" \
  -H "Authorization: Bearer $TOKEN"
```

**Delete File:**

```bash
curl -X DELETE "http://localhost:8080/api/v1/files?sandbox_id=abc123&path=/workspace/test.py" \
  -H "Authorization: Bearer $TOKEN"
```

## Configuration

### Environment Variables

**Gateway:**
| Variable | Default | Description |
|----------|---------|-------------|
| `JWT_SECRET` | `cloud-sandbox-secret` | JWT signing secret |
| `SCHEDULER_URL` | `http://localhost:9090` | Scheduler service URL |
| `SESSION_MANAGER_URL` | `http://localhost:9091` | Session manager URL |

**Scheduler:**
| Variable | Default | Description |
|----------|---------|-------------|
| `POOL_MIN_SIZE` | `5` | Minimum sandbox pool size |
| `POOL_MAX_SIZE` | `50` | Maximum sandbox pool size |
| `SANDBOX_IMAGE` | `python:3.11-slim` | Default container image |

**Session Manager:**
| Variable | Default | Description |
|----------|---------|-------------|
| `POSTGRES_HOST` | `localhost` | PostgreSQL host |
| `POSTGRES_PORT` | `5432` | PostgreSQL port |
| `REDIS_ADDR` | `localhost:6379` | Redis address |
| `MINIO_ENDPOINT` | `localhost:9000` | MinIO endpoint |

## Deployment

### Docker Compose (Development)

```bash
# Start all infrastructure services
docker compose up -d

# Build and run applications
make build
make run-all
```

### Kubernetes

```bash
# Deploy to development
make k8s-dev

# Deploy to production
make k8s-prod

# Delete deployment
make k8s-delete-dev
```

## Monitoring

The system exposes Prometheus metrics at `/metrics` endpoint on each service:

- **Sandbox metrics:** `sandboxes_total`, `sandboxes_active`, `sandboxes_idle`
- **Execution metrics:** `executions_total`, `execution_duration_seconds`
- **Session metrics:** `sessions_total`, `sessions_active`, `sessions_created_total`
- **HTTP metrics:** `http_requests_total`, `http_request_duration_seconds`

Grafana dashboards are available in `deploy/grafana/`.

## Troubleshooting

### Common Issues

**1. Sandbox acquisition timeout:**
- Check if Docker daemon is running
- Verify pool has available sandboxes: `curl http://localhost:9090/api/v1/sandbox/stats`

**2. Session manager connection errors:**
- Services work without PostgreSQL/Redis in development mode
- Check logs for connection warnings

**3. Code execution fails:**
- Verify sandbox is acquired and active
- Check container logs: `docker logs <container_id>`

### Logs

View service logs:

```bash
# If running with docker compose
docker compose logs -f gateway
docker compose logs -f scheduler
docker compose logs -f session-manager

# If running with Kubernetes
kubectl logs -f deployment/gateway -n cloud-sandbox
```

## Security Considerations

1. **JWT Secret:** Change the default JWT secret in production
2. **Network Isolation:** Sandboxes run in isolated Docker networks
3. **Resource Limits:** Configure CPU/memory limits per sandbox
4. **Secrets Management:** Use Kubernetes secrets or external secret managers
