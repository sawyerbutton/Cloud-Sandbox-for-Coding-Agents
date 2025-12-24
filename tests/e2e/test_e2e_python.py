#!/usr/bin/env python3
"""
End-to-End Integration Tests for Cloud Sandbox using Python SDK

This test suite validates the complete workflow from authentication
through code execution using the Cloud Sandbox Python SDK.

Requirements:
- Cloud Sandbox services running (Gateway, Scheduler, Session Manager)
- Python SDK installed: pip install -e sdk/python

Usage:
    python tests/e2e/test_e2e_python.py
    # or with pytest
    pytest tests/e2e/test_e2e_python.py -v
"""

import os
import sys
import time
import pytest

# Add SDK to path for testing
sys.path.insert(0, os.path.join(os.path.dirname(__file__), '../../sdk/python'))

from cloud_sandbox import Sandbox, SandboxClient, SandboxError


# Configuration
GATEWAY_URL = os.getenv("GATEWAY_URL", "http://localhost:8080")
TEST_USER_ID = os.getenv("TEST_USER_ID", "e2e-python-test-user")


class TestE2EAuthentication:
    """Test authentication flow"""

    def test_get_token(self):
        """Test JWT token generation"""
        client = SandboxClient(base_url=GATEWAY_URL)
        token_response = client.get_token(user_id=TEST_USER_ID, role="user")

        assert "access_token" in token_response
        assert "refresh_token" in token_response
        assert token_response["token_type"] == "Bearer"
        assert token_response["access_token"] is not None

    def test_invalid_request_without_user_id(self):
        """Test token request fails without user_id"""
        client = SandboxClient(base_url=GATEWAY_URL)
        with pytest.raises(SandboxError):
            client._request("POST", "/api/v1/auth/token", data={})


class TestE2ESessionManagement:
    """Test session management flow"""

    @pytest.fixture
    def authenticated_client(self):
        """Create an authenticated client"""
        client = SandboxClient(base_url=GATEWAY_URL)
        token_response = client.get_token(user_id=TEST_USER_ID)
        client.set_token(token_response["access_token"])
        return client

    def test_create_session(self, authenticated_client):
        """Test session creation"""
        session = authenticated_client.create_session(
            image="python:3.11-slim",
            cpu_count=2,
            memory_mb=1024,
        )

        assert session.id is not None
        assert session.status in ["active", "creating", "pending"]

        # Cleanup
        authenticated_client.delete_session(session.id)

    def test_list_sessions(self, authenticated_client):
        """Test listing user sessions"""
        # Create a session first
        session = authenticated_client.create_session()

        try:
            sessions = authenticated_client.list_sessions()
            assert len(sessions) >= 1
            assert any(s.id == session.id for s in sessions)
        finally:
            authenticated_client.delete_session(session.id)

    def test_get_session(self, authenticated_client):
        """Test getting session by ID"""
        session = authenticated_client.create_session()

        try:
            retrieved = authenticated_client.get_session(session.id)
            assert retrieved.id == session.id
            assert retrieved.user_id == TEST_USER_ID
        finally:
            authenticated_client.delete_session(session.id)

    def test_pause_resume_session(self, authenticated_client):
        """Test pausing and resuming a session"""
        session = authenticated_client.create_session()

        try:
            # Pause
            paused = authenticated_client.pause_session(session.id)
            assert paused.status == "paused"

            # Resume
            resumed = authenticated_client.resume_session(session.id)
            assert resumed.status == "active"
        finally:
            authenticated_client.delete_session(session.id)


class TestE2ESandboxExecution:
    """Test sandbox acquisition and code execution"""

    @pytest.fixture
    def sandbox(self):
        """Create a sandbox for testing"""
        sandbox = Sandbox.create(
            base_url=GATEWAY_URL,
            user_id=TEST_USER_ID,
            image="python:3.11-slim",
        )
        yield sandbox
        sandbox.destroy()

    def test_run_python_code(self, sandbox):
        """Test executing Python code"""
        result = sandbox.run_code(
            code="print('Hello from Python!')",
            language="python",
        )

        assert result.exit_code == 0
        assert "Hello from Python!" in result.stdout
        assert result.success is True

    def test_run_python_with_computation(self, sandbox):
        """Test executing Python code with computation"""
        result = sandbox.run_code(
            code="""
import math
result = math.factorial(10)
print(f"10! = {result}")
""",
            language="python",
        )

        assert result.exit_code == 0
        assert "10! = 3628800" in result.stdout

    def test_run_shell_command(self, sandbox):
        """Test executing shell command"""
        result = sandbox.run_command("echo 'Hello from shell!'")

        assert result.exit_code == 0
        assert "Hello from shell!" in result.stdout

    def test_run_command_list(self, sandbox):
        """Test executing command as list"""
        result = sandbox.run_command(["ls", "-la", "/"])

        assert result.exit_code == 0
        assert "root" in result.stdout or "bin" in result.stdout

    def test_execution_failure(self, sandbox):
        """Test handling of execution failure"""
        result = sandbox.run_code(
            code="import nonexistent_module",
            language="python",
        )

        assert result.exit_code != 0
        assert "ModuleNotFoundError" in result.stderr or "No module named" in result.stderr
        assert result.success is False


class TestE2EFileOperations:
    """Test file operations in sandbox"""

    @pytest.fixture
    def sandbox(self):
        """Create a sandbox for testing"""
        sandbox = Sandbox.create(
            base_url=GATEWAY_URL,
            user_id=TEST_USER_ID,
        )
        yield sandbox
        sandbox.destroy()

    def test_write_and_list_file(self, sandbox):
        """Test writing and listing files"""
        # Write a file
        content = "Hello, this is test content!"
        success = sandbox.write_file("/workspace/test.txt", content)
        assert success is True

        # List files
        files = sandbox.list_files("/workspace")
        file_names = [f.name for f in files]
        assert "test.txt" in file_names

    def test_write_and_execute_file(self, sandbox):
        """Test writing and executing a Python file"""
        # Write a Python script
        script = """
def greet(name):
    return f"Hello, {name}!"

print(greet("Cloud Sandbox"))
"""
        sandbox.write_file("/workspace/script.py", script)

        # Execute the script
        result = sandbox.run_command("python /workspace/script.py")

        assert result.exit_code == 0
        assert "Hello, Cloud Sandbox!" in result.stdout

    def test_delete_file(self, sandbox):
        """Test deleting a file"""
        # Write a file
        sandbox.write_file("/workspace/to_delete.txt", "delete me")

        # Verify it exists
        files_before = [f.name for f in sandbox.list_files("/workspace")]
        assert "to_delete.txt" in files_before

        # Delete the file
        success = sandbox.delete_file("/workspace/to_delete.txt")
        assert success is True

        # Verify it's gone
        files_after = [f.name for f in sandbox.list_files("/workspace")]
        assert "to_delete.txt" not in files_after


class TestE2EHighLevelSandbox:
    """Test high-level Sandbox class functionality"""

    def test_context_manager(self):
        """Test Sandbox as context manager"""
        with Sandbox.create(
            base_url=GATEWAY_URL,
            user_id=TEST_USER_ID,
        ) as sandbox:
            result = sandbox.run_code("print('Context manager works!')")
            assert result.exit_code == 0
            sandbox_id = sandbox.sandbox_id
            assert sandbox_id is not None

        # Sandbox should be released after context

    def test_pause_and_resume(self):
        """Test pause and resume workflow"""
        # Create and set up state
        sandbox = Sandbox.create(
            base_url=GATEWAY_URL,
            user_id=TEST_USER_ID,
        )

        # Write a file to preserve state
        sandbox.write_file("/workspace/state.txt", "preserved_state")

        # Pause the session
        session_id = sandbox.pause()
        assert session_id is not None

        # Resume with new sandbox
        resumed_sandbox = Sandbox.resume(
            session_id=session_id,
            base_url=GATEWAY_URL,
            user_id=TEST_USER_ID,
        )

        try:
            # Verify state was preserved
            files = resumed_sandbox.list_files("/workspace")
            file_names = [f.name for f in files]
            assert "state.txt" in file_names
        finally:
            resumed_sandbox.destroy()


class TestE2ESandboxPool:
    """Test sandbox pool operations"""

    @pytest.fixture
    def authenticated_client(self):
        """Create an authenticated client"""
        client = SandboxClient(base_url=GATEWAY_URL)
        token_response = client.get_token(user_id=TEST_USER_ID)
        client.set_token(token_response["access_token"])
        return client

    def test_get_pool_stats(self, authenticated_client):
        """Test getting sandbox pool statistics"""
        stats = authenticated_client.get_sandbox_stats()

        assert "total" in stats
        assert "available" in stats
        assert "in_use" in stats
        assert stats["total"] >= 0

    def test_acquire_and_release_sandbox(self, authenticated_client):
        """Test acquiring and releasing sandbox"""
        # Get initial stats
        initial_stats = authenticated_client.get_sandbox_stats()

        # Acquire sandbox
        sandbox_info = authenticated_client.acquire_sandbox()
        assert sandbox_info.sandbox_id is not None
        assert sandbox_info.status == "running"

        # Verify stats changed
        during_stats = authenticated_client.get_sandbox_stats()
        assert during_stats["in_use"] >= 1

        # Release sandbox
        success = authenticated_client.release_sandbox(sandbox_info.sandbox_id)
        assert success is True


def run_tests():
    """Run all e2e tests"""
    pytest.main([__file__, "-v", "--tb=short"])


if __name__ == "__main__":
    run_tests()
