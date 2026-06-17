"""Tests that pin the repository's public API surface across source files."""

from __future__ import annotations

import socket

import pytest

from backend.api_surface_contract import endpoint_keys


def test_sources_load_without_network(
    monkeypatch,
    reference_endpoints,
    openapi_endpoints,
    gateway_routes,
    frontend_paths,
):
    def blocked_socket(*_args, **_kwargs):
        raise AssertionError("API surface tests must not use the network")

    monkeypatch.setattr(socket, "socket", blocked_socket)

    assert reference_endpoints
    assert openapi_endpoints
    assert gateway_routes
    assert frontend_paths


def test_reference_and_openapi_drift_is_explicitly_tracked(reference_endpoints, openapi_endpoints):
    reference = endpoint_keys(reference_endpoints)
    openapi = endpoint_keys(openapi_endpoints)

    known_missing_from_openapi = {
        ("DELETE", "/orders/{param}"),
        ("GET", "/account/summary"),
        ("GET", "/account/transactions"),
        ("GET", "/market/candles"),
        ("GET", "/market/instruments/{param}"),
        ("GET", "/market/news"),
        ("GET", "/market/ticker"),
        ("GET", "/market/trades"),
        ("GET", "/orders"),
        ("GET", "/orders/{param}"),
        ("GET", "/positions"),
        ("GET", "/positions/{param}"),
        ("POST", "/orders"),
    }

    assert known_missing_from_openapi <= reference
    assert reference - openapi == known_missing_from_openapi


def test_gateway_implements_core_frontend_market_reads(gateway_routes, frontend_paths):
    gateway = endpoint_keys(gateway_routes)
    frontend = endpoint_keys(frontend_paths)

    core_market_reads = {
        ("GET", "/market/instruments"),
        ("GET", "/market/orderbook"),
        ("GET", "/market/ticker"),
        ("GET", "/market/candles"),
        ("GET", "/market/trades"),
        ("GET", "/market/news"),
    }

    assert core_market_reads <= frontend
    assert core_market_reads <= gateway


def test_frontend_drift_outside_gateway_is_known_and_bounded(gateway_routes, frontend_paths):
    gateway = endpoint_keys(gateway_routes)
    frontend = endpoint_keys(frontend_paths)

    known_frontend_only = {
        ("GET", "/account/{param}/summary"),
        ("GET", "/market/instruments/{param}"),
        ("GET", "/market/search"),
        ("GET", "/notifications"),
        ("GET", "/orders"),
        ("GET", "/portfolio/{param}"),
        ("GET", "/positions"),
        ("GET", "/user/preferences"),
        ("PUT", "/orders/{param}"),
        ("PUT", "/user/preferences"),
        ("POST", "/orders"),
        ("DELETE", "/orders/{param}"),
    }

    assert frontend - gateway == known_frontend_only


@pytest.mark.parametrize(
    ("method", "path", "payload"),
    [
        ("GET", "/market/instruments", None),
        ("POST", "/orders", {"symbol": "BTC/USD", "side": "buy"}),
        ("DELETE", "/orders/order-123", None),
    ],
)
def test_success_cases_for_documented_http_verbs(
    mock_api_client,
    auth_token,
    method,
    path,
    payload,
):
    response = mock_api_client.request(method, path, token=auth_token, payload=payload)

    assert 200 <= response.status_code < 300
    assert response.body["method"] == method


def test_missing_auth_token_returns_error(mock_api_client):
    response = mock_api_client.request("GET", "/orders")

    assert response.status_code == 401
    assert response.body["code"] == 4002


def test_unknown_endpoint_returns_not_found(mock_api_client, auth_token):
    response = mock_api_client.request("GET", "/market/unknown", token=auth_token)

    assert response.status_code == 404
    assert response.body["code"] == 4004


def test_missing_payload_returns_validation_error(mock_api_client, auth_token):
    response = mock_api_client.request("POST", "/orders", token=auth_token)

    assert response.status_code == 422
    assert response.body["code"] == 4001


def test_edge_case_payloads_are_accepted_by_offline_mock(
    mock_api_client,
    auth_token,
    sample_order_payload,
):
    payload = {
        **sample_order_payload,
        "client_note": "unicode snowman \\u2603",
        "metadata": {"source": "pytest", "large_note": "x" * 4096},
    }

    response = mock_api_client.request("POST", "/orders", token=auth_token, payload=payload)

    assert response.status_code == 201
    assert response.body["path"] == "/orders"


def test_internal_error_path_is_mocked_without_external_dependencies(mock_api_client, auth_token):
    response = mock_api_client.request(
        "POST",
        "/orders",
        token=auth_token,
        payload={"__force_internal_error__": True},
    )

    assert response.status_code == 500
    assert response.body["code"] == 5001


@pytest.mark.asyncio
async def test_async_request_wrapper(mock_api_client, auth_token):
    response = await mock_api_client.request_async("GET", "/market/news", token=auth_token)

    assert response.status_code == 200
    assert response.body["path"] == "/market/news"
