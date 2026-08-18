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

from app.agent import (
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
    check_system_health,
    list_cluster_resources,
    get_capacity_forecast,
    get_drift_status,
    list_incidents,
)


def test_root_agent_sub_agents() -> None:
    """Test that root_agent has exactly 10 sub-agents and their names match."""
    assert root_agent.sub_agents is not None
    assert len(root_agent.sub_agents) == 10

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
    actual_names = {agent.name for agent in root_agent.sub_agents}
    assert actual_names == expected_names


def test_devops_agent_tools() -> None:
    """Test that devops_agent has the correct tools assigned."""
    tools = devops_agent.tools
    assert tools is not None
    assert len(tools) == 4
    assert check_system_health in tools
    assert list_cluster_resources in tools
    assert get_capacity_forecast in tools
    assert list_incidents in tools


def test_kubernetes_agent_tools() -> None:
    """Test that kubernetes_agent has the correct tools assigned."""
    tools = kubernetes_agent.tools
    assert tools is not None
    assert len(tools) == 2
    assert list_cluster_resources in tools
    assert get_capacity_forecast in tools


def test_gitops_agent_tools() -> None:
    """Test that gitops_agent has the correct tools assigned."""
    tools = gitops_agent.tools
    assert tools is not None
    assert len(tools) == 1
    assert get_drift_status in tools


def test_agents_without_tools() -> None:
    """Test that agents without tools have empty tools list or None."""
    agents_without_tools = [
        architect_agent,
        backend_agent,
        frontend_agent,
        dba_agent,
        qa_agent,
        security_agent,
        reviewer_agent,
    ]
    for agent in agents_without_tools:
        assert agent.tools is None or len(agent.tools) == 0


def test_root_agent_instruction_keywords() -> None:
    """Test that root_agent instruction contains the routing table keywords."""
    instruction = root_agent.instruction
    keywords = [
        "PostgreSQL",
        "Docker",
        "Kubernetes",
        "Go",
        "HTML",
        "test",
        "PR",
        "security",
        "architecture",
        "GitOps",
        "health",
    ]
    for keyword in keywords:
        assert keyword in instruction
