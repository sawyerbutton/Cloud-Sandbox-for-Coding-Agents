"""Cloud Sandbox SDK Exceptions"""


class SandboxError(Exception):
    """Base exception for sandbox errors"""

    def __init__(self, message: str, status_code: int = None, response: dict = None):
        super().__init__(message)
        self.message = message
        self.status_code = status_code
        self.response = response


class AuthError(SandboxError):
    """Authentication error"""
    pass


class TimeoutError(SandboxError):
    """Execution timeout error"""
    pass


class NotFoundError(SandboxError):
    """Resource not found error"""
    pass


class RateLimitError(SandboxError):
    """Rate limit exceeded error"""
    pass


class ServiceUnavailableError(SandboxError):
    """Backend service unavailable error"""
    pass
