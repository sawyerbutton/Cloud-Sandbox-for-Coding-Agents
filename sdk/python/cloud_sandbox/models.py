"""Cloud Sandbox SDK Data Models"""

from dataclasses import dataclass, field
from datetime import datetime
from typing import Optional, Dict, List, Any


@dataclass
class Session:
    """Represents a sandbox session"""
    id: str
    user_id: str
    status: str
    sandbox_id: Optional[str] = None
    image: str = "python:3.11-slim"
    cpu_count: int = 2
    memory_mb: int = 2048
    workspace_url: Optional[str] = None
    created_at: Optional[datetime] = None
    updated_at: Optional[datetime] = None
    last_active_at: Optional[datetime] = None
    expires_at: Optional[datetime] = None
    metadata: Dict[str, str] = field(default_factory=dict)

    @classmethod
    def from_dict(cls, data: dict) -> "Session":
        """Create Session from API response"""
        return cls(
            id=data.get("id", ""),
            user_id=data.get("user_id", ""),
            status=data.get("status", ""),
            sandbox_id=data.get("sandbox_id"),
            image=data.get("image", "python:3.11-slim"),
            cpu_count=data.get("cpu_count", 2),
            memory_mb=data.get("memory_mb", 2048),
            workspace_url=data.get("workspace_url"),
            created_at=_parse_datetime(data.get("created_at")),
            updated_at=_parse_datetime(data.get("updated_at")),
            last_active_at=_parse_datetime(data.get("last_active_at")),
            expires_at=_parse_datetime(data.get("expires_at")),
            metadata=data.get("metadata", {}),
        )


@dataclass
class ExecResult:
    """Represents the result of code execution"""
    exit_code: int
    stdout: str
    stderr: str
    duration_ms: int
    timed_out: bool = False
    error: Optional[str] = None

    @classmethod
    def from_dict(cls, data: dict) -> "ExecResult":
        """Create ExecResult from API response"""
        return cls(
            exit_code=data.get("exit_code", -1),
            stdout=data.get("stdout", ""),
            stderr=data.get("stderr", ""),
            duration_ms=data.get("duration_ms", 0),
            timed_out=data.get("timed_out", False),
            error=data.get("error"),
        )

    @property
    def success(self) -> bool:
        """Check if execution was successful"""
        return self.exit_code == 0 and not self.timed_out and not self.error


@dataclass
class FileInfo:
    """Represents file metadata"""
    name: str
    path: str
    size: int
    is_dir: bool
    mod_time: Optional[datetime] = None

    @classmethod
    def from_dict(cls, data: dict) -> "FileInfo":
        """Create FileInfo from API response"""
        return cls(
            name=data.get("name", ""),
            path=data.get("path", ""),
            size=data.get("size", 0),
            is_dir=data.get("is_dir", False),
            mod_time=_parse_datetime(data.get("mod_time")),
        )


@dataclass
class SandboxInfo:
    """Represents sandbox information"""
    sandbox_id: str
    container_id: str
    status: str
    ip: Optional[str] = None

    @classmethod
    def from_dict(cls, data: dict) -> "SandboxInfo":
        """Create SandboxInfo from API response"""
        return cls(
            sandbox_id=data.get("sandbox_id", ""),
            container_id=data.get("container_id", ""),
            status=data.get("status", ""),
            ip=data.get("ip"),
        )


def _parse_datetime(value: Any) -> Optional[datetime]:
    """Parse datetime from various formats"""
    if value is None:
        return None
    if isinstance(value, datetime):
        return value
    if isinstance(value, str):
        # Try ISO format
        try:
            # Handle timezone suffix
            if value.endswith("Z"):
                value = value[:-1] + "+00:00"
            return datetime.fromisoformat(value)
        except ValueError:
            pass
    return None
