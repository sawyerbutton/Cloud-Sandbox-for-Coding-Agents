# Quick Start Guide

Get Cloud Sandbox running in 5 minutes.

## Prerequisites

- Docker installed and running
- Go 1.22+ installed
- curl for testing

## Step 1: Clone and Build

```bash
git clone https://github.com/cloud-sandbox/cloud-sandbox.git
cd cloud-sandbox
make build
```

## Step 2: Start Services

```bash
# Start scheduler (manages sandbox pool)
./bin/scheduler &

# Start session manager (handles sessions)
./bin/session-manager &

# Start gateway (unified API entry point)
./bin/gateway
```

Or use:
```bash
make run-all
```

## Step 3: Test the API

```bash
# Get authentication token
TOKEN=$(curl -s -X POST http://localhost:8080/api/v1/auth/token \
  -H "Content-Type: application/json" \
  -d '{"user_id": "test", "role": "user"}' | jq -r '.access_token')

# Acquire a sandbox
SANDBOX=$(curl -s -X POST http://localhost:8080/api/v1/sandbox/acquire \
  -H "Authorization: Bearer $TOKEN" | jq -r '.sandbox_id')

# Execute Python code
curl -s -X POST http://localhost:8080/api/v1/execute \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"sandbox_id\": \"$SANDBOX\", \"code\": \"print('Hello from Cloud Sandbox!')\", \"language\": \"python\"}"

# Release the sandbox
curl -s -X POST http://localhost:8080/api/v1/sandbox/release \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"sandbox_id\": \"$SANDBOX\"}"
```

## Step 4: Use the Python SDK

```bash
pip install -e sdk/python
```

```python
from cloud_sandbox import Sandbox

with Sandbox.create(
    base_url="http://localhost:8080",
    user_id="my-user",
) as sandbox:
    result = sandbox.run_code("print('Hello!')")
    print(result.stdout)
```

## Next Steps

- Read the [User Guide](user-guide.md) for detailed API documentation
- Check [Architecture](architecture-comparison.md) for system design
- See [Kubernetes Deployment](../deploy/k8s/) for production setup
