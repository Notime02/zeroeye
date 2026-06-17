"""Shared fixtures for offline API surface tests."""

from __future__ import annotations

from pathlib import Path

import pytest

from backend.api_surface_contract import (
    MockApiClient,
    extract_frontend_paths,
    extract_gateway_routes,
    extract_markdown_endpoints,
    extract_openapi_endpoints,
    read_text,
)


@pytest.fixture(scope="session")
def repo_root() -> Path:
    # Resolve from tests/ so the suite works from the repository root or pytest.
    return Path(__file__).resolve().parents[1]


@pytest.fixture(scope="session")
def api_reference_text(repo_root: Path) -> str:
    # Markdown reference is one of the public API surfaces clients read.
    return read_text(repo_root / "docs" / "API_REFERENCE.md")


@pytest.fixture(scope="session")
def openapi_text(repo_root: Path) -> str:
    # OpenAPI spec is parsed directly; no network or generator image is needed.
    return read_text(repo_root / "docs" / "openapi" / "v3.yaml")


@pytest.fixture(scope="session")
def gateway_text(repo_root: Path) -> str:
    # Go gateway registrations are the runtime HTTP surface for market routes.
    return read_text(repo_root / "market" / "gateway" / "api.go")


@pytest.fixture(scope="session")
def frontend_data_service_text(repo_root: Path) -> str:
    # Frontend data service calls show which endpoints the UI actually uses.
    return read_text(repo_root / "frontend" / "src" / "utils" / "dataService.ts")


@pytest.fixture(scope="session")
def reference_endpoints(api_reference_text: str):
    return extract_markdown_endpoints(api_reference_text)


@pytest.fixture(scope="session")
def openapi_endpoints(openapi_text: str):
    return extract_openapi_endpoints(openapi_text)


@pytest.fixture(scope="session")
def gateway_routes(gateway_text: str):
    return extract_gateway_routes(gateway_text)


@pytest.fixture(scope="session")
def frontend_paths(frontend_data_service_text: str):
    return extract_frontend_paths(frontend_data_service_text)


@pytest.fixture(scope="session")
def mock_api_client(reference_endpoints):
    # Use docs/API_REFERENCE.md as the broadest documented public contract.
    return MockApiClient(reference_endpoints)


@pytest.fixture(scope="session")
def auth_token() -> str:
    # Representative bearer token for protected API paths.
    return "test-access-token"


@pytest.fixture(scope="session")
def sample_order_payload() -> dict[str, object]:
    # Realistic order payload used by success, validation, and edge-case tests.
    return {
        "symbol": "BTC/USD",
        "side": "buy",
        "type": "limit",
        "quantity": "0.10",
        "price": "65000.00",
    }
