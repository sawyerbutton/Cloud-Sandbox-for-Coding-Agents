"""Tests for Cloud Sandbox SDK client"""

import pytest
from unittest.mock import Mock, patch, MagicMock
from cloud_sandbox.client import SandboxClient, Sandbox
from cloud_sandbox.models import Session, ExecResult, FileInfo, SandboxInfo
from cloud_sandbox.exceptions import (
    SandboxError,
    AuthError,
    TimeoutError,
    NotFoundError,
    RateLimitError,
    ServiceUnavailableError,
)


class TestSandboxClient:
    """Tests for SandboxClient class"""

    def test_init_defaults(self):
        """Test client initialization with defaults"""
        client = SandboxClient()
        assert client.base_url == "http://localhost:8080"
        assert client.token is None
        assert client.timeout == 300

    def test_init_custom(self):
        """Test client initialization with custom values"""
        client = SandboxClient(
            base_url="https://api.example.com",
            token="my-token",
            timeout=60,
        )
        assert client.base_url == "https://api.example.com"
        assert client.token == "my-token"
        assert client.timeout == 60

    def test_base_url_trailing_slash(self):
        """Test that trailing slash is stripped from base_url"""
        client = SandboxClient(base_url="http://localhost:8080/")
        assert client.base_url == "http://localhost:8080"

    def test_headers_without_token(self):
        """Test headers without token"""
        client = SandboxClient()
        headers = client._headers()
        assert headers == {"Content-Type": "application/json"}

    def test_headers_with_token(self):
        """Test headers with token"""
        client = SandboxClient(token="my-jwt-token")
        headers = client._headers()
        assert headers == {
            "Content-Type": "application/json",
            "Authorization": "Bearer my-jwt-token",
        }

    def test_set_token(self):
        """Test setting token"""
        client = SandboxClient()
        client.set_token("new-token")
        assert client.token == "new-token"

    @patch.object(SandboxClient, "_request")
    def test_get_token(self, mock_request):
        """Test get_token method"""
        mock_request.return_value = {
            "access_token": "jwt-token",
            "refresh_token": "refresh-token",
        }
        client = SandboxClient()
        result = client.get_token("user-1", "admin")

        mock_request.assert_called_once_with(
            "POST", "/api/v1/auth/token", data={"user_id": "user-1", "role": "admin"}
        )
        assert result["access_token"] == "jwt-token"

    @patch.object(SandboxClient, "_request")
    def test_create_session(self, mock_request):
        """Test create_session method"""
        mock_request.return_value = {
            "id": "sess-123",
            "user_id": "user-1",
            "status": "active",
            "image": "python:3.11-slim",
        }
        client = SandboxClient(token="token")
        session = client.create_session(image="python:3.11-slim", cpu_count=2)

        assert isinstance(session, Session)
        assert session.id == "sess-123"
        assert session.status == "active"

    @patch.object(SandboxClient, "_request")
    def test_list_sessions(self, mock_request):
        """Test list_sessions method"""
        mock_request.return_value = {
            "sessions": [
                {"id": "sess-1", "user_id": "user-1", "status": "active"},
                {"id": "sess-2", "user_id": "user-1", "status": "paused"},
            ]
        }
        client = SandboxClient(token="token")
        sessions = client.list_sessions()

        assert len(sessions) == 2
        assert all(isinstance(s, Session) for s in sessions)
        assert sessions[0].id == "sess-1"
        assert sessions[1].status == "paused"

    @patch.object(SandboxClient, "_request")
    def test_acquire_sandbox(self, mock_request):
        """Test acquire_sandbox method"""
        mock_request.return_value = {
            "sandbox_id": "sb-123",
            "container_id": "abc123",
            "status": "running",
        }
        client = SandboxClient(token="token")
        sandbox = client.acquire_sandbox()

        assert isinstance(sandbox, SandboxInfo)
        assert sandbox.sandbox_id == "sb-123"

    @patch.object(SandboxClient, "_request")
    def test_execute_code(self, mock_request):
        """Test execute method with code"""
        mock_request.return_value = {
            "exit_code": 0,
            "stdout": "Hello!\n",
            "stderr": "",
            "duration_ms": 100,
        }
        client = SandboxClient(token="token")
        result = client.execute(
            sandbox_id="sb-123",
            code="print('Hello!')",
            language="python",
            timeout=30,
        )

        assert isinstance(result, ExecResult)
        assert result.exit_code == 0
        assert result.stdout == "Hello!\n"
        assert result.success is True

    @patch.object(SandboxClient, "_request")
    def test_execute_command(self, mock_request):
        """Test execute method with command"""
        mock_request.return_value = {
            "exit_code": 0,
            "stdout": "file1.txt\nfile2.txt\n",
            "stderr": "",
            "duration_ms": 50,
        }
        client = SandboxClient(token="token")
        result = client.execute(
            sandbox_id="sb-123",
            command=["ls", "-la"],
        )

        assert result.stdout == "file1.txt\nfile2.txt\n"

    @patch.object(SandboxClient, "_request")
    def test_list_files(self, mock_request):
        """Test list_files method"""
        mock_request.return_value = {
            "files": [
                {"name": "file1.py", "path": "/workspace/file1.py", "size": 100, "is_dir": False},
                {"name": "src", "path": "/workspace/src", "size": 0, "is_dir": True},
            ]
        }
        client = SandboxClient(token="token")
        files = client.list_files("sb-123", "/workspace")

        assert len(files) == 2
        assert all(isinstance(f, FileInfo) for f in files)
        assert files[0].name == "file1.py"
        assert files[1].is_dir is True


class TestSandboxClientErrors:
    """Tests for error handling in SandboxClient"""

    @patch("requests.Session.request")
    def test_auth_error(self, mock_request):
        """Test AuthError on 401 response"""
        mock_response = Mock()
        mock_response.status_code = 401
        mock_response.json.return_value = {"error": "invalid token"}
        mock_request.return_value = mock_response

        client = SandboxClient(token="bad-token")
        with pytest.raises(AuthError) as exc_info:
            client.list_sessions()
        assert exc_info.value.status_code == 401

    @patch("requests.Session.request")
    def test_not_found_error(self, mock_request):
        """Test NotFoundError on 404 response"""
        mock_response = Mock()
        mock_response.status_code = 404
        mock_response.json.return_value = {"error": "session not found"}
        mock_request.return_value = mock_response

        client = SandboxClient(token="token")
        with pytest.raises(NotFoundError):
            client.get_session("non-existent")

    @patch("requests.Session.request")
    def test_rate_limit_error(self, mock_request):
        """Test RateLimitError on 429 response"""
        mock_response = Mock()
        mock_response.status_code = 429
        mock_response.json.return_value = {"error": "rate limit exceeded"}
        mock_request.return_value = mock_response

        client = SandboxClient(token="token")
        with pytest.raises(RateLimitError):
            client.list_sessions()

    @patch("requests.Session.request")
    def test_service_unavailable(self, mock_request):
        """Test ServiceUnavailableError on 502 response"""
        mock_response = Mock()
        mock_response.status_code = 502
        mock_response.json.return_value = {"error": "backend unavailable"}
        mock_request.return_value = mock_response

        client = SandboxClient(token="token")
        with pytest.raises(ServiceUnavailableError):
            client.list_sessions()

    @patch("requests.Session.request")
    def test_timeout_error(self, mock_request):
        """Test TimeoutError on request timeout"""
        import requests.exceptions

        mock_request.side_effect = requests.exceptions.Timeout("timeout")

        client = SandboxClient(token="token", timeout=1)
        with pytest.raises(TimeoutError):
            client.list_sessions()

    @patch("requests.Session.request")
    def test_connection_error(self, mock_request):
        """Test ServiceUnavailableError on connection error"""
        import requests.exceptions

        mock_request.side_effect = requests.exceptions.ConnectionError("connection refused")

        client = SandboxClient(token="token")
        with pytest.raises(ServiceUnavailableError):
            client.list_sessions()


class TestSandbox:
    """Tests for high-level Sandbox class"""

    def test_init_with_token(self):
        """Test Sandbox initialization with token"""
        sandbox = Sandbox(base_url="http://localhost:8080", token="my-token")
        assert sandbox.client.token == "my-token"
        assert sandbox.auto_release is True

    @patch.object(SandboxClient, "get_token")
    def test_init_with_user_id(self, mock_get_token):
        """Test Sandbox initialization with user_id"""
        mock_get_token.return_value = {"access_token": "generated-token"}
        sandbox = Sandbox(base_url="http://localhost:8080", user_id="user-1")

        mock_get_token.assert_called_once_with("user-1")
        assert sandbox.client.token == "generated-token"

    @patch.object(SandboxClient, "get_token")
    @patch.object(SandboxClient, "create_session")
    @patch.object(SandboxClient, "acquire_sandbox")
    def test_create(self, mock_acquire, mock_create_session, mock_get_token):
        """Test Sandbox.create class method"""
        mock_get_token.return_value = {"access_token": "token"}
        mock_create_session.return_value = Session(
            id="sess-123", user_id="user-1", status="active"
        )
        mock_acquire.return_value = SandboxInfo(
            sandbox_id="sb-456", container_id="abc", status="running"
        )

        sandbox = Sandbox.create(
            base_url="http://localhost:8080",
            user_id="user-1",
            image="python:3.11-slim",
        )

        assert sandbox.session_id == "sess-123"
        assert sandbox.sandbox_id == "sb-456"

    def test_run_code_without_sandbox(self):
        """Test run_code raises error when sandbox not acquired"""
        sandbox = Sandbox(token="token")
        with pytest.raises(SandboxError) as exc_info:
            sandbox.run_code("print('hi')")
        assert "Sandbox not acquired" in str(exc_info.value)

    @patch.object(SandboxClient, "execute")
    def test_run_code(self, mock_execute):
        """Test run_code method"""
        mock_execute.return_value = ExecResult(
            exit_code=0, stdout="output", stderr="", duration_ms=100
        )

        sandbox = Sandbox(token="token")
        sandbox._sandbox = SandboxInfo(
            sandbox_id="sb-123", container_id="abc", status="running"
        )

        result = sandbox.run_code("print('hi')", language="python", timeout=30)

        mock_execute.assert_called_once_with(
            sandbox_id="sb-123",
            code="print('hi')",
            language="python",
            timeout=30,
        )
        assert result.stdout == "output"

    @patch.object(SandboxClient, "execute")
    def test_run_command_string(self, mock_execute):
        """Test run_command with string command"""
        mock_execute.return_value = ExecResult(
            exit_code=0, stdout="output", stderr="", duration_ms=50
        )

        sandbox = Sandbox(token="token")
        sandbox._sandbox = SandboxInfo(
            sandbox_id="sb-123", container_id="abc", status="running"
        )

        sandbox.run_command("ls -la")

        # String command should be wrapped with bash -c
        mock_execute.assert_called_once()
        call_args = mock_execute.call_args
        assert call_args.kwargs["command"] == ["bash", "-c", "ls -la"]

    @patch.object(SandboxClient, "execute")
    def test_run_command_list(self, mock_execute):
        """Test run_command with list command"""
        mock_execute.return_value = ExecResult(
            exit_code=0, stdout="output", stderr="", duration_ms=50
        )

        sandbox = Sandbox(token="token")
        sandbox._sandbox = SandboxInfo(
            sandbox_id="sb-123", container_id="abc", status="running"
        )

        sandbox.run_command(["ls", "-la"])

        mock_execute.assert_called_once()
        call_args = mock_execute.call_args
        assert call_args.kwargs["command"] == ["ls", "-la"]

    @patch.object(SandboxClient, "pause_session")
    @patch.object(SandboxClient, "release_sandbox")
    def test_pause(self, mock_release, mock_pause):
        """Test pause method"""
        mock_pause.return_value = Session(
            id="sess-123", user_id="user-1", status="paused"
        )

        sandbox = Sandbox(token="token")
        sandbox._session = Session(id="sess-123", user_id="user-1", status="active")
        sandbox._sandbox = SandboxInfo(
            sandbox_id="sb-456", container_id="abc", status="running"
        )

        session_id = sandbox.pause()

        assert session_id == "sess-123"
        mock_pause.assert_called_once_with("sess-123")
        mock_release.assert_called_once_with("sb-456")
        assert sandbox._sandbox is None

    @patch.object(SandboxClient, "delete_session")
    @patch.object(SandboxClient, "release_sandbox")
    def test_destroy(self, mock_release, mock_delete):
        """Test destroy method"""
        sandbox = Sandbox(token="token")
        sandbox._session = Session(id="sess-123", user_id="user-1", status="active")
        sandbox._sandbox = SandboxInfo(
            sandbox_id="sb-456", container_id="abc", status="running"
        )

        sandbox.destroy()

        mock_release.assert_called_once_with("sb-456")
        mock_delete.assert_called_once_with("sess-123")
        assert sandbox._sandbox is None
        assert sandbox._session is None

    @patch.object(SandboxClient, "release_sandbox")
    def test_context_manager_auto_release(self, mock_release):
        """Test context manager auto-releases sandbox"""
        sandbox = Sandbox(token="token", auto_release=True)
        sandbox._sandbox = SandboxInfo(
            sandbox_id="sb-123", container_id="abc", status="running"
        )

        with sandbox:
            pass

        mock_release.assert_called_once_with("sb-123")
        assert sandbox._sandbox is None

    @patch.object(SandboxClient, "release_sandbox")
    def test_context_manager_no_auto_release(self, mock_release):
        """Test context manager respects auto_release=False"""
        sandbox = Sandbox(token="token", auto_release=False)
        sandbox._sandbox = SandboxInfo(
            sandbox_id="sb-123", container_id="abc", status="running"
        )

        with sandbox:
            pass

        mock_release.assert_not_called()
        assert sandbox._sandbox is not None
