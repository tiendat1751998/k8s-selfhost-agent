package helm

import (
	"context"
	"fmt"
	"strings"
	"time"

	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/release"
)

// InstallRequest represents parameters for installing a Helm chart.
type InstallRequest struct {
	ReleaseName     string                 `json:"releaseName"`
	Chart           string                 `json:"chart"`
	Repo            string                 `json:"repo"`
	Version         string                 `json:"version"`
	Namespace       string                 `json:"namespace"`
	Values          map[string]interface{} `json:"values"`
	CreateNamespace bool                   `json:"createNamespace"`
	Wait            bool                   `json:"wait"`
	Timeout         time.Duration          `json:"timeout"`
}

// UpgradeRequest represents parameters for upgrading a Helm release.
type UpgradeRequest struct {
	ReleaseName string                 `json:"releaseName"`
	Chart       string                 `json:"chart"`
	Repo        string                 `json:"repo"`
	Version     string                 `json:"version"`
	Namespace   string                 `json:"namespace"`
	Values      map[string]interface{} `json:"values"`
	ResetValues bool                   `json:"resetValues"`
	ReuseValues bool                   `json:"reuseValues"`
	Wait        bool                   `json:"wait"`
	Timeout     time.Duration          `json:"timeout"`
}


// InstallRelease installs a Helm chart into the specified cluster and namespace.
func (m *ReleaseManager) InstallRelease(ctx context.Context, clusterID string, req InstallRequest) (*release.Release, error) {
	if strings.TrimSpace(req.ReleaseName) == "" {
		return nil, fmt.Errorf("release name is required")
	}
	if strings.TrimSpace(req.Chart) == "" {
		return nil, fmt.Errorf("chart name is required")
	}
	ns := req.Namespace
	if strings.TrimSpace(ns) == "" {
		ns = "default"
	}

	actionConfig, err := m.getActionConfig(ctx, clusterID, ns)
	if err != nil {
		return nil, err
	}

	install := action.NewInstall(actionConfig)
	install.ReleaseName = req.ReleaseName
	install.Namespace = ns
	install.CreateNamespace = req.CreateNamespace
	install.Version = req.Version
	if req.Wait {
		install.Wait = true
	}
	if req.Timeout > 0 {
		install.Timeout = req.Timeout
	} else {
		install.Timeout = 5 * time.Minute
	}

	chartRef := req.Chart
	cpo := &install.ChartPathOptions
	cpo.Version = req.Version
	if req.Repo != "" {
		if strings.HasPrefix(req.Repo, "http://") || strings.HasPrefix(req.Repo, "https://") || strings.HasPrefix(req.Repo, "oci://") {
			cpo.RepoURL = req.Repo
		} else if !strings.Contains(req.Chart, "/") {
			chartRef = fmt.Sprintf("%s/%s", req.Repo, req.Chart)
		}
	}

	m.mu.RLock()
	chartPath, err := cpo.LocateChart(chartRef, m.settings)
	m.mu.RUnlock()
	if err != nil {
		return nil, fmt.Errorf("locating chart %s: %w", chartRef, err)
	}

	chartRequested, err := loader.Load(chartPath)
	if err != nil {
		return nil, fmt.Errorf("loading chart %s from %s: %w", chartRef, chartPath, err)
	}

	if chartRequested.Metadata.Type != "" && chartRequested.Metadata.Type != "application" {
		return nil, fmt.Errorf("%s charts are not installable", chartRequested.Metadata.Type)
	}

	values := req.Values
	if values == nil {
		values = make(map[string]interface{})
	}

	rel, err := install.RunWithContext(ctx, chartRequested, values)
	if err != nil {
		return nil, fmt.Errorf("installing release %s: %w", req.ReleaseName, err)
	}

	return rel, nil
}

// UpgradeRelease upgrades an existing Helm release with new values, chart, or version.
func (m *ReleaseManager) UpgradeRelease(ctx context.Context, clusterID string, req UpgradeRequest) (*release.Release, error) {
	if strings.TrimSpace(req.ReleaseName) == "" {
		return nil, fmt.Errorf("release name is required")
	}
	ns := req.Namespace
	if strings.TrimSpace(ns) == "" {
		ns = "default"
	}

	actionConfig, err := m.getActionConfig(ctx, clusterID, ns)
	if err != nil {
		return nil, err
	}

	upgrade := action.NewUpgrade(actionConfig)
	upgrade.Namespace = ns
	upgrade.Version = req.Version
	upgrade.ResetValues = req.ResetValues
	upgrade.ReuseValues = req.ReuseValues
	if req.Wait {
		upgrade.Wait = true
	}
	if req.Timeout > 0 {
		upgrade.Timeout = req.Timeout
	} else {
		upgrade.Timeout = 5 * time.Minute
	}

	var chartRequested *chart.Chart
	if req.Chart != "" {
		chartRef := req.Chart
		cpo := &upgrade.ChartPathOptions
		cpo.Version = req.Version
		if req.Repo != "" {
			if strings.HasPrefix(req.Repo, "http://") || strings.HasPrefix(req.Repo, "https://") || strings.HasPrefix(req.Repo, "oci://") {
				cpo.RepoURL = req.Repo
			} else if !strings.Contains(req.Chart, "/") {
				chartRef = fmt.Sprintf("%s/%s", req.Repo, req.Chart)
			}
		}

		m.mu.RLock()
		chartPath, err := cpo.LocateChart(chartRef, m.settings)
		m.mu.RUnlock()
		if err != nil {
			return nil, fmt.Errorf("locating chart %s: %w", chartRef, err)
		}

		chartRequested, err = loader.Load(chartPath)
		if err != nil {
			return nil, fmt.Errorf("loading chart %s from %s: %w", chartRef, chartPath, err)
		}
	} else {
		getAction := action.NewGet(actionConfig)
		existing, err := getAction.Run(req.ReleaseName)
		if err != nil {
			return nil, fmt.Errorf("getting existing release %s: %w", req.ReleaseName, err)
		}
		if existing.Chart == nil {
			return nil, fmt.Errorf("existing release %s has no chart", req.ReleaseName)
		}
		chartRequested = existing.Chart
	}

	values := req.Values
	if values == nil {
		values = make(map[string]interface{})
	}

	rel, err := upgrade.RunWithContext(ctx, req.ReleaseName, chartRequested, values)
	if err != nil {
		return nil, fmt.Errorf("upgrading release %s: %w", req.ReleaseName, err)
	}

	return rel, nil
}

// RollbackRelease rolls back a release to a previous revision.
func (m *ReleaseManager) RollbackRelease(ctx context.Context, clusterID, name, namespace string, revision int) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("release name is required")
	}

	actionConfig, err := m.getActionConfig(ctx, clusterID, namespace)
	if err != nil {
		return err
	}

	rollbackAction := action.NewRollback(actionConfig)
	rollbackAction.Version = revision
	rollbackAction.Wait = true
	rollbackAction.Timeout = 5 * time.Minute

	if err := rollbackAction.Run(name); err != nil {
		return fmt.Errorf("rolling back release %s to revision %d: %w", name, revision, err)
	}
	return nil
}

// UninstallRelease uninstalls a named release from the cluster.
func (m *ReleaseManager) UninstallRelease(ctx context.Context, clusterID, name, namespace string) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("release name is required")
	}

	actionConfig, err := m.getActionConfig(ctx, clusterID, namespace)
	if err != nil {
		return err
	}

	uninstallAction := action.NewUninstall(actionConfig)
	uninstallAction.Wait = true
	uninstallAction.Timeout = 5 * time.Minute

	if _, err := uninstallAction.Run(name); err != nil {
		return fmt.Errorf("uninstalling release %s: %w", name, err)
	}
	return nil
}
