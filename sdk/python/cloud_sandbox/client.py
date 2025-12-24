"""Cloud Sandbox SDK Client"""

import time
from typing import Optional, Dict, List, Any, Union
import requests

from .models import Session, ExecResult, FileInfo, SandboxInfo
from .exceptions import (
    SandboxError,
    AuthError,
    TimeoutError,
    NotFoundError,
    RateLimitError,
    ServiceUnavailableError,
)


class SandboxClient:
    """Low-level client for Cloud Sandbox API"""

    def __init__(
        self,
        base_url: str = "http://localhost:8080",
        token: Optional[str] = None,
        timeout: int = 300,
    ):
        """
        Initialize the sandbox client.

        Args:
            base_url: The base URL of the Cloud Sandbox API
            token: JWT access token for authentication
            timeout: Default request timeout in seconds
        """
        self.base_url = base_url.rstrip("/")
        self.token = token
        self.timeout = timeout
        self.session = requests.Session()

    def _headers(self) -> Dict[str, str]:
        """Get request headers"""
        headers = {"Content-Type": "application/json"}
        if self.token:
            headers["Authorization"] = f"Bearer {self.token}"
        return headers

    def _request(
        self,
        method: str,
        path: str,
        data: Optional[Dict] = None,
        params: Optional[Dict] = None,
        timeout: Optional[int] = None,
    ) -> Dict[str, Any]:
        """Make an API request"""
        url = f"{self.base_url}{path}"
        timeout = timeout or self.timeout

        try:
            response = self.session.request(
                method=method,
                url=url,
                json=data,
                params=params,
                headers=self._headers(),
                timeout=timeout,
            )
        except requests.exceptions.Timeout:
            raise TimeoutError(f"Request timed out after {timeout}s")
        except requests.exceptions.ConnectionError as e:
            raise ServiceUnavailableError(f"Connection error: {e}")

        return self._handle_response(response)

    def _handle_response(self, response: requests.Response) -> Dict[str, Any]:
        """Handle API response"""
        if response.status_code == 401:
            raise AuthError("Authentication failed", response.status_code)
        if response.status_code == 404:
            raise NotFoundError("Resource not found", response.status_code)
        if response.status_code == 429:
            raise RateLimitError("Rate limit exceeded", response.status_code)
        if response.status_code == 502:
            raise ServiceUnavailableError("Backend service unavailable", response.status_code)

        try:
            data = response.json()
        except ValueError:
            data = {"raw": response.text}

        if response.status_code >= 400:
            error_msg = data.get("error", data.get("message", "Unknown error"))
            raise SandboxError(error_msg, response.status_code, data)

        return data

    # Auth methods
    def get_token(self, user_id: str, role: str = "user") -> Dict[str, str]:
        """Get a new access token"""
        return self._request("POST", "/api/v1/auth/token", data={"user_id": user_id, "role": role})

    def set_token(self, token: str):
        """Set the access token"""
        self.token = token

    # Session methods
    def create_session(
        self,
        image: str = None,
        cpu_count: int = None,
        memory_mb: int = None,
        ttl_hours: int = None,
    ) -> Session:
        """Create a new session"""
        data = {}
        if image:
            data["image"] = image
        if cpu_count:
            data["cpu_count"] = cpu_count
        if memory_mb:
            data["memory_mb"] = memory_mb
        if ttl_hours:
            data["ttl"] = ttl_hours * 3600 * 1000000000  # Convert to nanoseconds

        response = self._request("POST", "/api/v1/sessions", data=data)
        return Session.from_dict(response)

    def get_session(self, session_id: str) -> Session:
        """Get session by ID"""
        response = self._request("GET", f"/api/v1/sessions/{session_id}")
        return Session.from_dict(response)

    def list_sessions(self) -> List[Session]:
        """List all sessions for the current user"""
        response = self._request("GET", "/api/v1/sessions")
        sessions = response.get("sessions", [])
        return [Session.from_dict(s) for s in sessions]

    def delete_session(self, session_id: str) -> bool:
        """Delete a session"""
        response = self._request("DELETE", f"/api/v1/sessions/{session_id}")
        return response.get("success", False)

    def pause_session(self, session_id: str) -> Session:
        """Pause a session"""
        response = self._request("POST", f"/api/v1/sessions/{session_id}/pause")
        return Session.from_dict(response)

    def resume_session(self, session_id: str) -> Session:
        """Resume a paused session"""
        response = self._request("POST", f"/api/v1/sessions/{session_id}/resume")
        return Session.from_dict(response)

    # Sandbox methods
    def acquire_sandbox(self) -> SandboxInfo:
        """Acquire a sandbox from the pool"""
        response = self._request("POST", "/api/v1/sandbox/acquire")
        return SandboxInfo.from_dict(response)

    def release_sandbox(self, sandbox_id: str) -> bool:
        """Release a sandbox back to the pool"""
        response = self._request("POST", "/api/v1/sandbox/release", data={"sandbox_id": sandbox_id})
        return response.get("success", False)

    def get_sandbox_stats(self) -> Dict[str, int]:
        """Get sandbox pool statistics"""
        return self._request("GET", "/api/v1/sandbox/stats")

    # Execution methods
    def execute(
        self,
        sandbox_id: str,
        code: str = None,
        language: str = "python",
        command: List[str] = None,
        work_dir: str = None,
        env: Dict[str, str] = None,
        timeout: int = None,
    ) -> ExecResult:
        """Execute code in a sandbox"""
        data = {"sandbox_id": sandbox_id}

        if code:
            data["code"] = code
            data["language"] = language
        if command:
            data["command"] = command
        if work_dir:
            data["work_dir"] = work_dir
        if env:
            data["env"] = env
        if timeout:
            data["timeout"] = timeout

        response = self._request("POST", "/api/v1/execute", data=data, timeout=timeout or 300)
        return ExecResult.from_dict(response)

    # File methods
    def list_files(self, sandbox_id: str, path: str = "/workspace") -> List[FileInfo]:
        """List files in a sandbox directory"""
        response = self._request(
            "GET", "/api/v1/files", params={"sandbox_id": sandbox_id, "path": path}
        )
        files = response.get("files", [])
        return [FileInfo.from_dict(f) for f in files]

    def write_file(self, sandbox_id: str, path: str, content: Union[str, bytes]) -> bool:
        """Write content to a file in the sandbox"""
        if isinstance(content, bytes):
            content = content.decode("utf-8")

        response = self._request(
            "PUT",
            "/api/v1/files",
            data={"sandbox_id": sandbox_id, "path": path, "content": content},
        )
        return response.get("success", False)

    def delete_file(self, sandbox_id: str, path: str) -> bool:
        """Delete a file from the sandbox"""
        response = self._request(
            "DELETE", "/api/v1/files", params={"sandbox_id": sandbox_id, "path": path}
        )
        return response.get("success", False)


class Sandbox:
    """High-level sandbox interface for easy usage"""

    def __init__(
        self,
        base_url: str = "http://localhost:8080",
        token: Optional[str] = None,
        user_id: Optional[str] = None,
        auto_release: bool = True,
    ):
        """
        Initialize a sandbox.

        Args:
            base_url: The base URL of the Cloud Sandbox API
            token: JWT access token (if not provided, will request one)
            user_id: User ID for token generation (required if token not provided)
            auto_release: Whether to auto-release sandbox on exit
        """
        self.client = SandboxClient(base_url, token)
        self.auto_release = auto_release

        self._session: Optional[Session] = None
        self._sandbox: Optional[SandboxInfo] = None

        # Get token if not provided
        if not token and user_id:
            token_response = self.client.get_token(user_id)
            self.client.set_token(token_response["access_token"])

    @classmethod
    def create(
        cls,
        base_url: str = "http://localhost:8080",
        user_id: str = "default-user",
        image: str = None,
        cpu_count: int = None,
        memory_mb: int = None,
        **kwargs,
    ) -> "Sandbox":
        """
        Create a new sandbox and acquire resources.

        Args:
            base_url: The base URL of the Cloud Sandbox API
            user_id: User ID for authentication
            image: Container image to use
            cpu_count: Number of CPUs
            memory_mb: Memory in MB
            **kwargs: Additional arguments passed to Sandbox.__init__

        Returns:
            A ready-to-use Sandbox instance
        """
        sandbox = cls(base_url=base_url, user_id=user_id, **kwargs)

        # Create session
        sandbox._session = sandbox.client.create_session(
            image=image, cpu_count=cpu_count, memory_mb=memory_mb
        )

        # Acquire sandbox
        sandbox._sandbox = sandbox.client.acquire_sandbox()

        return sandbox

    @classmethod
    def resume(cls, session_id: str, base_url: str = "http://localhost:8080", **kwargs) -> "Sandbox":
        """
        Resume an existing session.

        Args:
            session_id: The session ID to resume
            base_url: The base URL of the Cloud Sandbox API
            **kwargs: Additional arguments passed to Sandbox.__init__

        Returns:
            A resumed Sandbox instance
        """
        sandbox = cls(base_url=base_url, **kwargs)

        # Resume session
        sandbox._session = sandbox.client.resume_session(session_id)

        # Acquire sandbox
        sandbox._sandbox = sandbox.client.acquire_sandbox()

        return sandbox

    @property
    def session_id(self) -> Optional[str]:
        """Get the current session ID"""
        return self._session.id if self._session else None

    @property
    def sandbox_id(self) -> Optional[str]:
        """Get the current sandbox ID"""
        return self._sandbox.sandbox_id if self._sandbox else None

    def run_code(self, code: str, language: str = "python", timeout: int = 300) -> ExecResult:
        """
        Execute code in the sandbox.

        Args:
            code: The code to execute
            language: Programming language (python, bash, node, etc.)
            timeout: Execution timeout in seconds

        Returns:
            ExecResult with stdout, stderr, exit_code, etc.
        """
        if not self._sandbox:
            raise SandboxError("Sandbox not acquired")

        return self.client.execute(
            sandbox_id=self.sandbox_id,
            code=code,
            language=language,
            timeout=timeout,
        )

    def run_command(self, command: Union[str, List[str]], timeout: int = 300) -> ExecResult:
        """
        Execute a shell command in the sandbox.

        Args:
            command: Command string or list of arguments
            timeout: Execution timeout in seconds

        Returns:
            ExecResult with stdout, stderr, exit_code, etc.
        """
        if not self._sandbox:
            raise SandboxError("Sandbox not acquired")

        if isinstance(command, str):
            command = ["bash", "-c", command]

        return self.client.execute(
            sandbox_id=self.sandbox_id,
            command=command,
            timeout=timeout,
        )

    def write_file(self, path: str, content: Union[str, bytes]) -> bool:
        """Write content to a file in the sandbox"""
        if not self._sandbox:
            raise SandboxError("Sandbox not acquired")
        return self.client.write_file(self.sandbox_id, path, content)

    def list_files(self, path: str = "/workspace") -> List[FileInfo]:
        """List files in the sandbox"""
        if not self._sandbox:
            raise SandboxError("Sandbox not acquired")
        return self.client.list_files(self.sandbox_id, path)

    def delete_file(self, path: str) -> bool:
        """Delete a file from the sandbox"""
        if not self._sandbox:
            raise SandboxError("Sandbox not acquired")
        return self.client.delete_file(self.sandbox_id, path)

    def pause(self) -> str:
        """
        Pause the session and release sandbox.

        Returns:
            Session ID for resuming later
        """
        if not self._session:
            raise SandboxError("No active session")

        # Pause session (saves workspace)
        self._session = self.client.pause_session(self._session.id)

        # Release sandbox
        if self._sandbox:
            self.client.release_sandbox(self.sandbox_id)
            self._sandbox = None

        return self._session.id

    def destroy(self):
        """Destroy the sandbox and session completely"""
        if self._sandbox:
            self.client.release_sandbox(self.sandbox_id)
            self._sandbox = None

        if self._session:
            self.client.delete_session(self._session.id)
            self._session = None

    def __enter__(self) -> "Sandbox":
        return self

    def __exit__(self, exc_type, exc_val, exc_tb):
        if self.auto_release and self._sandbox:
            self.client.release_sandbox(self.sandbox_id)
            self._sandbox = None
