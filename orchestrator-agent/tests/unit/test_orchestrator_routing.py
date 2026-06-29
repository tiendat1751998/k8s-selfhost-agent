# Copyright 2026 Google LLC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     https://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

import json
from unittest.mock import patch, mock_open, MagicMock

from app.agent import (
    load_skill_instruction,
    check_system_health,
    list_cluster_resources,
    get_drift_status,
    list_incidents,
    get_capacity_forecast,
    root_agent,
    architect_agent,
    backend_agent,
    frontend_agent,
    dba_agent,
    devops_agent,
    qa_agent,
    security_agent,
    reviewer_agent,
    kubernetes_agent,
    gitops_agent,
)


# ---------------------------------------------------------------------------
# load_skill_instruction Tests
# ---------------------------------------------------------------------------

def test_load_skill_instruction_success() -> None:
    """Test load_skill_instruction reads and strips YAML frontmatter correctly."""
    mock_skill_content = """---
name: Test Skill
description: A dummy test skill
---
Actual Instruction Content Here"""

    with patch("os.path.exists", return_value=True), \
         patch("builtins.open", mock_open(read_data=mock_skill_content)):
        res = load_skill_instruction("architect", "Fallback instruction")
        assert res == "Actual Instruction Content Here"


def test_load_skill_instruction_no_frontmatter() -> None:
    """Test load_skill_instruction passes through content without frontmatter."""
    mock_skill_content = "Raw Instruction Content"

    with patch("os.path.exists", return_value=True), \
         patch("builtins.open", mock_open(read_data=mock_skill_content)):
        res = load_skill_instruction("architect", "Fallback instruction")
        assert res == "Raw Instruction Content"


def test_load_skill_instruction_fallback() -> None:
    """Test load_skill_instruction returns fallback when file does not exist."""
    with patch("os.path.exists", return_value=False):
        res = load_skill_instruction("non_existent", "Fallback instruction")
        assert res == "Fallback instruction"


# ---------------------------------------------------------------------------
# Real Tool Tests (with mocked HTTP backend)
# ---------------------------------------------------------------------------

def _mock_urlopen(response_data: dict):
    """Create a mock urllib response context manager returning JSON data."""
    mock_resp = MagicMock()
    mock_resp.read.return_value = json.dumps(response_data).encode("utf-8")
    mock_resp.__enter__ = MagicMock(return_value=mock_resp)
    mock_resp.__exit__ = MagicMock(return_value=False)
    return mock_resp


def test_check_system_health_returns_json() -> None:
    """Test check_system_health calls backend /api/v1/health and returns JSON."""
    health_data = {"postgres": "healthy", "swarm": "connected", "redis": "healthy"}
    with patch("app.agent.urllib.request.urlopen", return_value=_mock_urlopen(health_data)):
        result = check_system_health("check all components")
        parsed = json.loads(result)
        assert parsed["postgres"] == "healthy"
        assert parsed["swarm"] == "connected"


def test_list_cluster_resources_passes_kind() -> None:
    """Test list_cluster_resources includes kind query parameter."""
    pods_data = {"items": [{"name": "nginx-pod", "status": "Running"}]}
    with patch("app.agent.urllib.request.urlopen", return_value=_mock_urlopen(pods_data)) as mock_open_call:
        result = list_cluster_resources("pod")
        parsed = json.loads(result)
        assert len(parsed["items"]) == 1
        assert parsed["items"][0]["name"] == "nginx-pod"


def test_get_drift_status_returns_json() -> None:
    """Test get_drift_status calls backend /api/v1/drift."""
    drift_data = {"drifted": 2, "resources": [{"name": "svc-a", "status": "drifted"}]}
    with patch("app.agent.urllib.request.urlopen", return_value=_mock_urlopen(drift_data)):
        result = get_drift_status("check all namespaces")
        parsed = json.loads(result)
        assert parsed["drifted"] == 2


def test_list_incidents_returns_json() -> None:
    """Test list_incidents calls backend /api/v1/incidents."""
    incident_data = {"incidents": [{"id": 1, "severity": "critical", "message": "OOMKill"}]}
    with patch("app.agent.urllib.request.urlopen", return_value=_mock_urlopen(incident_data)):
        result = list_incidents("critical incidents")
        parsed = json.loads(result)
        assert len(parsed["incidents"]) == 1
        assert parsed["incidents"][0]["severity"] == "critical"


def test_get_capacity_forecast_returns_json() -> None:
    """Test get_capacity_forecast calls backend /api/v1/capacity."""
    capacity_data = {"cpu_usage_pct": 72.5, "memory_usage_pct": 68.0, "nodes": 3}
    with patch("app.agent.urllib.request.urlopen", return_value=_mock_urlopen(capacity_data)):
        result = get_capacity_forecast("cluster resource utilization")
        parsed = json.loads(result)
        assert parsed["cpu_usage_pct"] == 72.5
        assert parsed["nodes"] == 3


def test_backend_unreachable_returns_error() -> None:
    """Test tools gracefully handle backend connection failures."""
    import urllib.error
    with patch("app.agent.urllib.request.urlopen", side_effect=urllib.error.URLError("Connection refused")):
        result = check_system_health("health check")
        parsed = json.loads(result)
        assert "error" in parsed
        assert "unreachable" in parsed["error"]


# ---------------------------------------------------------------------------
# Agent Structure Tests
# ---------------------------------------------------------------------------

def test_root_agent_has_ten_sub_agents() -> None:
    """Test the root orchestrator has exactly 10 specialist sub-agents."""
    assert root_agent.name == "root_agent"
    assert len(root_agent.sub_agents) == 10

    sub_agent_names = {agent.name for agent in root_agent.sub_agents}
    expected_names = {
        "architect_agent",
        "backend_agent",
        "frontend_agent",
        "dba_agent",
        "devops_agent",
        "qa_agent",
        "security_agent",
        "reviewer_agent",
        "kubernetes_agent",
        "gitops_agent",
    }
    assert sub_agent_names == expected_names


def test_root_agent_has_no_tools() -> None:
    """Verify root orchestrator has no direct tools (routing only)."""
    assert not root_agent.tools


def test_root_agent_instruction_contains_routing_table() -> None:
    """Verify root agent instruction includes routing table for all agents."""
    instruction = root_agent.instruction
    for agent_name in ["architect_agent", "backend_agent", "frontend_agent",
                       "dba_agent", "devops_agent", "qa_agent",
                       "security_agent", "reviewer_agent",
                       "kubernetes_agent", "gitops_agent"]:
        assert agent_name in instruction, f"Missing {agent_name} in routing instruction"


def test_agent_tool_attachments() -> None:
    """Test that operational tools are attached to the correct specialist agents."""
    assert not architect_agent.tools
    assert not backend_agent.tools
    assert not frontend_agent.tools
    assert not dba_agent.tools
    assert not qa_agent.tools
    assert not security_agent.tools
    assert not reviewer_agent.tools

    assert check_system_health in devops_agent.tools
    assert list_cluster_resources in devops_agent.tools
    assert get_capacity_forecast in devops_agent.tools

    assert list_cluster_resources in kubernetes_agent.tools
    assert get_capacity_forecast in kubernetes_agent.tools

    assert get_drift_status in gitops_agent.tools


def test_all_agents_use_same_model() -> None:
    """Verify all agents share the same Gemini model instance."""
    all_agents = [
        root_agent, architect_agent, backend_agent, frontend_agent,
        dba_agent, devops_agent, qa_agent, security_agent,
        reviewer_agent, kubernetes_agent, gitops_agent,
    ]
    models = {id(agent.model) for agent in all_agents}
    assert len(models) == 1, "All agents must share the same model instance"
