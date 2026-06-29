"""MCP Server exposing K8S Self-Healing Platform tools as Model Context Protocol resources.

This server provides MCP-compatible tool endpoints for querying the K8S Control Plane
backend API, enabling external agents and LLM-based systems to access live cluster
health, resource listings, drift detection, incident data, and capacity forecasts.

Demonstrates Course Concept #2 (MCP Server) for the Kaggle Capstone submission.
"""

import json
import os
import urllib.request
import urllib.error
from typing import Any

BACKEND_BASE_URL = os.getenv("K8S_BACKEND_URL", "http://localhost:8080")


def _backend_get(path: str) -> dict[str, Any]:
    """Execute a GET request against the K8S Control Plane backend API."""
    url = f"{BACKEND_BASE_URL}{path}"
    req = urllib.request.Request(url, headers={"Accept": "application/json"})
    try:
        with urllib.request.urlopen(req, timeout=10) as resp:
            return json.loads(resp.read().decode("utf-8"))
    except urllib.error.URLError as exc:
        return {"error": f"Backend unreachable at {url}: {exc.reason}"}
    except Exception as exc:
        return {"error": f"Request to {url} failed: {exc}"}


TOOL_DEFINITIONS: list[dict[str, Any]] = [
    {
        "name": "check_system_health",
        "description": (
            "Check the real-time health status of all platform components including "
            "PostgreSQL, Docker Swarm, Kubernetes API, Redis, and WebSocket connections."
        ),
        "inputSchema": {
            "type": "object",
            "properties": {
                "query": {
                    "type": "string",
                    "description": "A descriptive query about which components to check.",
                }
            },
            "required": ["query"],
        },
    },
    {
        "name": "list_cluster_resources",
        "description": (
            "List Kubernetes or Docker Swarm resources from the active cluster. "
            "Returns live resources (Pods, Services, Deployments, Nodes, ReplicaSets)."
        ),
        "inputSchema": {
            "type": "object",
            "properties": {
                "kind": {
                    "type": "string",
                    "description": "The resource kind: pod, service, deployment, node, replicaset.",
                    "enum": ["pod", "service", "deployment", "node", "replicaset"],
                }
            },
            "required": ["kind"],
        },
    },
    {
        "name": "get_drift_status",
        "description": (
            "Check GitOps configuration drift between the Git baseline and live cluster state. "
            "Detects services that have drifted from their declared state."
        ),
        "inputSchema": {
            "type": "object",
            "properties": {
                "query": {
                    "type": "string",
                    "description": "A descriptive query about which resources to check for drift.",
                }
            },
            "required": ["query"],
        },
    },
    {
        "name": "list_incidents",
        "description": (
            "List active incidents and their AI-powered root cause analysis (RCA). "
            "Includes severity levels, affected resources, and automated RCA results."
        ),
        "inputSchema": {
            "type": "object",
            "properties": {
                "query": {
                    "type": "string",
                    "description": "A descriptive query to filter incidents by severity or resource.",
                }
            },
            "required": ["query"],
        },
    },
    {
        "name": "get_capacity_forecast",
        "description": (
            "Get cluster capacity utilization and resource forecasting data. "
            "Returns CPU, memory, and storage metrics with predictive forecasts."
        ),
        "inputSchema": {
            "type": "object",
            "properties": {
                "query": {
                    "type": "string",
                    "description": "A descriptive query about which cluster to analyze.",
                }
            },
            "required": ["query"],
        },
    },
]

TOOL_HANDLERS: dict[str, str] = {
    "check_system_health": "/api/v1/health",
    "list_cluster_resources": "/api/v1/explorer",
    "get_drift_status": "/api/v1/drift",
    "list_incidents": "/api/v1/incidents",
    "get_capacity_forecast": "/api/v1/capacity",
}


def handle_tool_call(tool_name: str, arguments: dict[str, Any]) -> dict[str, Any]:
    """Route an MCP tool call to the appropriate backend endpoint.

    Args:
        tool_name: The name of the MCP tool being called.
        arguments: The arguments passed to the tool.

    Returns:
        A dictionary containing the tool execution result.
    """
    endpoint = TOOL_HANDLERS.get(tool_name)
    if endpoint is None:
        return {"error": f"Unknown tool: {tool_name}"}

    path = endpoint
    if tool_name == "list_cluster_resources" and "kind" in arguments:
        path = f"{endpoint}?kind={arguments['kind']}"

    return _backend_get(path)


def list_tools() -> list[dict[str, Any]]:
    """Return the list of available MCP tool definitions."""
    return TOOL_DEFINITIONS


def handle_mcp_request(request: dict[str, Any]) -> dict[str, Any]:
    """Process an incoming MCP JSON-RPC request.

    Supports the following MCP methods:
    - tools/list: Returns available tool definitions
    - tools/call: Executes a tool and returns results

    Args:
        request: A JSON-RPC 2.0 request dictionary.

    Returns:
        A JSON-RPC 2.0 response dictionary.
    """
    method = request.get("method", "")
    request_id = request.get("id")

    if method == "tools/list":
        return {
            "jsonrpc": "2.0",
            "id": request_id,
            "result": {"tools": list_tools()},
        }

    if method == "tools/call":
        params = request.get("params", {})
        tool_name = params.get("name", "")
        arguments = params.get("arguments", {})
        result = handle_tool_call(tool_name, arguments)
        return {
            "jsonrpc": "2.0",
            "id": request_id,
            "result": {
                "content": [
                    {
                        "type": "text",
                        "text": json.dumps(result, indent=2),
                    }
                ]
            },
        }

    return {
        "jsonrpc": "2.0",
        "id": request_id,
        "error": {
            "code": -32601,
            "message": f"Method not found: {method}",
        },
    }
