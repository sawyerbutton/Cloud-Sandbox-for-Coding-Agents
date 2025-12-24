# Cloud Sandbox Python SDK

Python client library for interacting with Cloud Sandbox - isolated execution environments for AI coding agents.

## Installation

```bash
pip install cloud-sandbox
```

Or install from source:

```bash
cd sdk/python
pip install -e .
```

## Quick Start

### Basic Usage

```python
from cloud_sandbox import Sandbox

# Create a sandbox with automatic resource management
with Sandbox.create(
    base_url="http://localhost:8080",
    user_id="my-user-id",
    image="python:3.11-slim",
) as sandbox:
    # Execute Python code
    result = sandbox.run_code("""
import sys
print(f"Python version: {sys.version}")
print("Hello from sandbox!")
""")
    print(result.stdout)

    # Run shell commands
    result = sandbox.run_command("ls -la /workspace")
    print(result.stdout)

    # File operations
    sandbox.write_file("/workspace/test.py", "print('Hello!')")
    files = sandbox.list_files("/workspace")
    for f in files:
        print(f"{f.name} - {f.size} bytes")
```

### Session Management

```python
from cloud_sandbox import Sandbox

# Create and pause session for later
sandbox = Sandbox.create(base_url="http://localhost:8080", user_id="user123")
sandbox.run_code("x = 42")  # Set up state

# Pause session (saves workspace state)
session_id = sandbox.pause()
print(f"Session paused: {session_id}")

# Later: Resume the session
sandbox = Sandbox.resume(
    session_id=session_id,
    base_url="http://localhost:8080",
    user_id="user123"
)
result = sandbox.run_code("print(x)")  # State restored
print(result.stdout)  # Output: 42

# Clean up when done
sandbox.destroy()
```

### Low-Level Client

For more control, use the `SandboxClient` directly:

```python
from cloud_sandbox import SandboxClient

client = SandboxClient(base_url="http://localhost:8080")

# Get authentication token
token_response = client.get_token(user_id="my-user", role="user")
client.set_token(token_response["access_token"])

# Create a session
session = client.create_session(
    image="python:3.11-slim",
    cpu_count=2,
    memory_mb=2048,
    ttl_hours=24
)

# Acquire sandbox from pool
sandbox_info = client.acquire_sandbox()

# Execute code
result = client.execute(
    sandbox_id=sandbox_info.sandbox_id,
    code="print('Hello!')",
    language="python",
    timeout=30
)
print(f"Exit code: {result.exit_code}")
print(f"Output: {result.stdout}")

# Release sandbox back to pool
client.release_sandbox(sandbox_info.sandbox_id)

# Delete session when done
client.delete_session(session.id)
```

## API Reference

### Sandbox Class

High-level interface for easy sandbox usage.

#### Class Methods

- `Sandbox.create(base_url, user_id, image=None, cpu_count=None, memory_mb=None, **kwargs)` - Create a new sandbox
- `Sandbox.resume(session_id, base_url, **kwargs)` - Resume an existing session

#### Instance Methods

- `run_code(code, language="python", timeout=300)` - Execute code
- `run_command(command, timeout=300)` - Execute shell command
- `write_file(path, content)` - Write file to sandbox
- `list_files(path="/workspace")` - List files in directory
- `delete_file(path)` - Delete a file
- `pause()` - Pause session and save state
- `destroy()` - Destroy sandbox and session

### SandboxClient Class

Low-level client for direct API access.

#### Authentication

- `get_token(user_id, role="user")` - Get JWT access token
- `set_token(token)` - Set access token for requests

#### Session Management

- `create_session(image, cpu_count, memory_mb, ttl_hours)` - Create session
- `get_session(session_id)` - Get session details
- `list_sessions()` - List user's sessions
- `delete_session(session_id)` - Delete session
- `pause_session(session_id)` - Pause session
- `resume_session(session_id)` - Resume session

#### Sandbox Operations

- `acquire_sandbox()` - Acquire sandbox from pool
- `release_sandbox(sandbox_id)` - Release sandbox to pool
- `get_sandbox_stats()` - Get pool statistics

#### Execution

- `execute(sandbox_id, code, language, command, work_dir, env, timeout)` - Execute code/command

#### File Operations

- `list_files(sandbox_id, path)` - List files
- `write_file(sandbox_id, path, content)` - Write file
- `delete_file(sandbox_id, path)` - Delete file

### Data Models

- `Session` - Session information (id, status, image, resources, timestamps)
- `ExecResult` - Execution result (exit_code, stdout, stderr, duration_ms)
- `FileInfo` - File metadata (name, path, size, is_dir, mod_time)
- `SandboxInfo` - Sandbox information (sandbox_id, container_id, status, ip)

### Exceptions

- `SandboxError` - Base exception
- `AuthError` - Authentication failed
- `TimeoutError` - Request/execution timeout
- `NotFoundError` - Resource not found
- `RateLimitError` - Rate limit exceeded
- `ServiceUnavailableError` - Backend unavailable

## Development

```bash
# Install dev dependencies
pip install -e ".[dev]"

# Run tests
pytest

# Format code
black cloud_sandbox tests

# Lint
ruff check cloud_sandbox tests

# Type check
mypy cloud_sandbox
```

## License

MIT License - see LICENSE file for details.
