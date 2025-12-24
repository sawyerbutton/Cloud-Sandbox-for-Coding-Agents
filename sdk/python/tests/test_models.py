"""Tests for Cloud Sandbox SDK models"""

import pytest
from datetime import datetime
from cloud_sandbox.models import Session, ExecResult, FileInfo, SandboxInfo


class TestSession:
    """Tests for Session model"""

    def test_from_dict_minimal(self):
        """Test Session creation with minimal data"""
        data = {
            "id": "sess-123",
            "user_id": "user-1",
            "status": "active",
        }
        session = Session.from_dict(data)

        assert session.id == "sess-123"
        assert session.user_id == "user-1"
        assert session.status == "active"
        assert session.sandbox_id is None
        assert session.image == "python:3.11-slim"
        assert session.cpu_count == 2
        assert session.memory_mb == 2048

    def test_from_dict_full(self):
        """Test Session creation with full data"""
        data = {
            "id": "sess-456",
            "user_id": "user-2",
            "status": "paused",
            "sandbox_id": "sb-789",
            "image": "node:18-slim",
            "cpu_count": 4,
            "memory_mb": 4096,
            "workspace_url": "s3://bucket/workspace.tar.gz",
            "created_at": "2024-01-15T10:30:00Z",
            "metadata": {"key": "value"},
        }
        session = Session.from_dict(data)

        assert session.id == "sess-456"
        assert session.sandbox_id == "sb-789"
        assert session.image == "node:18-slim"
        assert session.cpu_count == 4
        assert session.memory_mb == 4096
        assert session.workspace_url == "s3://bucket/workspace.tar.gz"
        assert session.created_at is not None
        assert session.metadata == {"key": "value"}


class TestExecResult:
    """Tests for ExecResult model"""

    def test_from_dict_success(self):
        """Test ExecResult for successful execution"""
        data = {
            "exit_code": 0,
            "stdout": "Hello, World!\n",
            "stderr": "",
            "duration_ms": 150,
        }
        result = ExecResult.from_dict(data)

        assert result.exit_code == 0
        assert result.stdout == "Hello, World!\n"
        assert result.stderr == ""
        assert result.duration_ms == 150
        assert result.timed_out is False
        assert result.error is None
        assert result.success is True

    def test_from_dict_failure(self):
        """Test ExecResult for failed execution"""
        data = {
            "exit_code": 1,
            "stdout": "",
            "stderr": "Error: file not found\n",
            "duration_ms": 50,
            "error": "execution failed",
        }
        result = ExecResult.from_dict(data)

        assert result.exit_code == 1
        assert result.stderr == "Error: file not found\n"
        assert result.error == "execution failed"
        assert result.success is False

    def test_from_dict_timeout(self):
        """Test ExecResult for timed out execution"""
        data = {
            "exit_code": -1,
            "stdout": "partial output...",
            "stderr": "",
            "duration_ms": 30000,
            "timed_out": True,
        }
        result = ExecResult.from_dict(data)

        assert result.timed_out is True
        assert result.success is False


class TestFileInfo:
    """Tests for FileInfo model"""

    def test_from_dict_file(self):
        """Test FileInfo for a regular file"""
        data = {
            "name": "script.py",
            "path": "/workspace/script.py",
            "size": 1024,
            "is_dir": False,
            "mod_time": "2024-01-15T12:00:00Z",
        }
        file_info = FileInfo.from_dict(data)

        assert file_info.name == "script.py"
        assert file_info.path == "/workspace/script.py"
        assert file_info.size == 1024
        assert file_info.is_dir is False
        assert file_info.mod_time is not None

    def test_from_dict_directory(self):
        """Test FileInfo for a directory"""
        data = {
            "name": "src",
            "path": "/workspace/src",
            "size": 0,
            "is_dir": True,
        }
        file_info = FileInfo.from_dict(data)

        assert file_info.name == "src"
        assert file_info.is_dir is True


class TestSandboxInfo:
    """Tests for SandboxInfo model"""

    def test_from_dict(self):
        """Test SandboxInfo creation"""
        data = {
            "sandbox_id": "sb-123",
            "container_id": "abc123def456",
            "status": "running",
            "ip": "172.17.0.5",
        }
        info = SandboxInfo.from_dict(data)

        assert info.sandbox_id == "sb-123"
        assert info.container_id == "abc123def456"
        assert info.status == "running"
        assert info.ip == "172.17.0.5"
