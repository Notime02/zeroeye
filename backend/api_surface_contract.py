"""Offline API surface helpers for pytest.

The repository's public API surface is split across OpenAPI docs, markdown
reference text, a Go gateway, and TypeScript clients. These helpers parse those
sources without importing service runtimes or opening network sockets.
"""

from __future__ import annotations

from dataclasses import dataclass
import re
from pathlib import Path
from typing import Iterable


HTTP_METHODS = {"GET", "POST", "PUT", "PATCH", "DELETE"}


@dataclass(frozen=True, order=True)
class Endpoint:
    """A normalized public API endpoint."""

    method: str
    path: str
    source: str

    @property
    def key(self) -> tuple[str, str]:
        return self.method, self.path


@dataclass(frozen=True)
class MockResponse:
    """Small response object returned by the offline mock client."""

    status_code: int
    body: dict[str, object]


def read_text(path: Path) -> str:
    """Read source text with replacement so unusual comments never break tests."""

    return path.read_text(encoding="utf-8", errors="replace")


def normalize_path(path: str) -> str:
    """Normalize version prefixes, query strings, and template expressions."""

    path = path.split("?", 1)[0]
    path = re.sub(r"^/api/v\d+", "", path)
    path = re.sub(r"\$\{[^}]+\}", "{param}", path)
    path = re.sub(r"\{[^}]+\}", "{param}", path)
    path = re.sub(r":([A-Za-z_][A-Za-z0-9_]*)", r"{\1}", path)
    return path or "/"


def extract_markdown_endpoints(text: str) -> set[Endpoint]:
    """Extract endpoint headings from docs/API_REFERENCE.md."""

    pattern = re.compile(r"^###\s+(GET|POST|PUT|PATCH|DELETE)\s+([^\s]+)", re.MULTILINE)
    return {
        Endpoint(method, normalize_path(path), "api-reference")
        for method, path in pattern.findall(text)
    }


def extract_openapi_endpoints(text: str) -> set[Endpoint]:
    """Extract method/path pairs from the OpenAPI path map."""

    endpoints: set[Endpoint] = set()
    current_path: str | None = None
    path_pattern = re.compile(r"^\s{2}(/[^:\s]+):\s*$")
    method_pattern = re.compile(r"^\s{4}(get|post|put|patch|delete):\s*$")

    for line in text.splitlines():
        path_match = path_pattern.match(line)
        if path_match:
            current_path = normalize_path(path_match.group(1))
            continue

        method_match = method_pattern.match(line)
        if current_path and method_match:
            endpoints.add(Endpoint(method_match.group(1).upper(), current_path, "openapi"))

    return endpoints


def extract_gateway_routes(text: str) -> set[Endpoint]:
    """Extract Go gateway route registrations and their guarded HTTP methods."""

    routes = re.findall(r'HandleFunc\("([^"]+)",\s*g\.(handle\w+)\(\)\)', text)
    endpoints: set[Endpoint] = set()
    for raw_path, handler_name in routes:
        path = normalize_path(raw_path)
        method = _handler_method(text, handler_name)
        endpoints.add(Endpoint(method, path, "go-gateway"))
    return endpoints


def extract_frontend_paths(text: str) -> set[Endpoint]:
    """Extract TypeScript client paths with the HTTP method implied by the call."""

    endpoints: set[Endpoint] = set()

    for path in re.findall(r"fetchWithCache\(\s*`([^`]+)`", text):
        endpoints.add(Endpoint("GET", normalize_path(path), "frontend"))
    for path in re.findall(r"fetchWithCache\(\s*'([^']+)'", text):
        endpoints.add(Endpoint("GET", normalize_path(path), "frontend"))
    for path in re.findall(r"\b(?:let|const)\s+url\s*=\s*`([^`]+)`", text):
        endpoints.add(Endpoint("GET", normalize_path(path), "frontend"))
    for path in re.findall(r"\b(?:let|const)\s+url\s*=\s*'([^']+)'", text):
        endpoints.add(Endpoint("GET", normalize_path(path), "frontend"))

    call_methods = {
        "post": "POST",
        "put": "PUT",
        "del": "DELETE",
        "get": "GET",
    }
    for call, method in call_methods.items():
        for path in re.findall(rf"\b{call}\(\s*`([^`]+)`", text):
            endpoints.add(Endpoint(method, normalize_path(path), "frontend"))
        for path in re.findall(rf"\b{call}\(\s*'([^']+)'", text):
            endpoints.add(Endpoint(method, normalize_path(path), "frontend"))

    return endpoints


def endpoint_keys(endpoints: Iterable[Endpoint]) -> set[tuple[str, str]]:
    """Return just method/path keys for set comparisons."""

    return {endpoint.key for endpoint in endpoints}


class MockApiClient:
    """Deterministic offline client for documented API behavior tests."""

    def __init__(self, endpoints: Iterable[Endpoint]):
        self._supported = endpoint_keys(endpoints)

    def request(
        self,
        method: str,
        path: str,
        *,
        token: str | None = None,
        payload: dict[str, object] | None = None,
    ) -> MockResponse:
        """Return success and common API errors without touching the network."""

        method = method.upper()
        path = normalize_path(path)

        if payload and payload.get("__force_internal_error__"):
            return MockResponse(500, {"code": 5001, "message": "Internal server error"})

        if not self._is_supported(method, path):
            return MockResponse(404, {"code": 4004, "message": "Resource not found"})

        if not path.startswith("/auth/") and token is None:
            return MockResponse(401, {"code": 4002, "message": "Authentication required"})

        if method in {"POST", "PUT", "PATCH"} and payload is None:
            return MockResponse(422, {"code": 4001, "message": "Invalid request payload"})

        status_code = 201 if method == "POST" else 200
        return MockResponse(status_code, {"ok": True, "method": method, "path": path})

    async def request_async(
        self,
        method: str,
        path: str,
        *,
        token: str | None = None,
        payload: dict[str, object] | None = None,
    ) -> MockResponse:
        """Async wrapper for pytest-asyncio coverage."""

        return self.request(method, path, token=token, payload=payload)

    def _is_supported(self, method: str, path: str) -> bool:
        if (method, path) in self._supported:
            return True

        for supported_method, supported_path in self._supported:
            if supported_method != method:
                continue
            if _template_to_regex(supported_path).fullmatch(path):
                return True
        return False


def _handler_method(text: str, handler_name: str) -> str:
    method_guard = re.search(
        rf"func \(g \*Gateway\) {re.escape(handler_name)}\(\) http\.HandlerFunc \{{"
        r"(?P<body>.*?)\n\}",
        text,
        re.DOTALL,
    )
    if not method_guard:
        return "GET"

    body = method_guard.group("body")
    match = re.search(r"r\.Method != http\.Method([A-Za-z]+)", body)
    if not match:
        return "GET"

    method = match.group(1).upper()
    return method if method in HTTP_METHODS else "GET"


def _template_to_regex(path: str) -> re.Pattern[str]:
    escaped = re.escape(path)
    escaped = escaped.replace(r"\{param\}", r"[^/]+")
    return re.compile(escaped)
