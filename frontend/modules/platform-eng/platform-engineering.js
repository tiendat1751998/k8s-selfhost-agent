/**
 * Platform Engineering Module
 * Service catalog, golden templates, and deployment blueprints.
 */
(function (global) {
  'use strict';

  var catalog = [
    { id: 'tpl-web', name: 'Web Application', icon: '🌐', description: 'Standard web app with ingress, HPA, and health checks', deploys: 45, tags: ['web', 'ingress', 'hpa'] },
    { id: 'tpl-api', name: 'REST API Service', icon: '🔗', description: 'Stateless API with rate limiting and circuit breaker', deploys: 32, tags: ['api', 'rest', 'stateless'] },
    { id: 'tpl-worker', name: 'Background Worker', icon: '⚙️', description: 'Queue consumer with auto-scaling and retry logic', deploys: 18, tags: ['worker', 'queue', 'async'] },
    { id: 'tpl-cron', name: 'CronJob', icon: '⏰', description: 'Scheduled batch job with concurrency control', deploys: 12, tags: ['cron', 'batch', 'scheduled'] },
    { id: 'tpl-stateful', name: 'StatefulSet', icon: '💾', description: 'Stateful service with persistent volumes and ordered deployment', deploys: 8, tags: ['stateful', 'pvc', 'database'] },
    { id: 'tpl-ml', name: 'ML Pipeline', icon: '🤖', description: 'ML training job with GPU support and model registry', deploys: 5, tags: ['ml', 'gpu', 'training'] }
  ];

  var goldenTemplates = [
    { name: 'Production Deployment', env: 'production', resources: { cpu: '500m-2000m', memory: '256Mi-1Gi' }, features: ['HPA', 'PDB', 'NetworkPolicy', 'ResourceQuota'], approved: true },
    { name: 'Staging Deployment', env: 'staging', resources: { cpu: '250m-1000m', memory: '128Mi-512Mi' }, features: ['HPA', 'ResourceQuota'], approved: true },
    { name: 'Development Deployment', env: 'development', resources: { cpu: '100m-500m', memory: '64Mi-256Mi' }, features: ['No HPA', 'Relaxed limits'], approved: true },
    { name: 'High-Availability Service', env: 'production', resources: { cpu: '1000m-4000m', memory: '512Mi-2Gi' }, features: ['HPA', 'PDB', 'Anti-affinity', 'Multi-zone', 'NetworkPolicy'], approved: true }
  ];

  var blueprints = [
    { name: 'E-Commerce Backend', services: ['API Gateway', 'Payment Service', 'Order Service', 'Inventory API', 'User Service'], infra: ['PostgreSQL', 'Redis', 'RabbitMQ'] },
    { name: 'Data Pipeline', services: ['Ingestion Worker', 'Transform Service', 'Aggregation Engine', 'Report Generator'], infra: ['Kafka', 'ClickHouse', 'MinIO'] },
    { name: 'ML Platform', services: ['Training Scheduler', 'Model Server', 'Feature Store API', 'Experiment Tracker'], infra: ['PostgreSQL', 'Redis', 'S3'] }
  ];

  function renderCatalog() {
    var container = document.getElementById('pe-catalog-grid');
    if (!container) return;
    container.innerHTML = catalog.map(function(t) {
      return '<div class="panel" style="padding:var(--space-md);cursor:pointer;transition:transform 0.2s;" onmouseover="this.style.transform=\'translateY(-2px)\'" onmouseout="this.style.transform=\'none\'">'
        + '<div style="font-size:32px;margin-bottom:8px;">' + t.icon + '</div>'
        + '<h4 style="margin:0 0 4px;font-size:14px;">' + t.name + '</h4>'
        + '<p style="font-size:12px;color:var(--color-muted);margin:0 0 8px;">' + t.description + '</p>'
        + '<div style="display:flex;flex-wrap:wrap;gap:4px;margin-bottom:8px;">' + t.tags.map(function(tag){ return '<span style="font-size:10px;background:var(--color-surface);border:1px solid var(--color-hairline);padding:1px 6px;border-radius:3px;">' + tag + '</span>'; }).join('') + '</div>'
        + '<div style="display:flex;justify-content:space-between;align-items:center;">'
        + '<span style="font-size:11px;color:var(--color-muted);">' + t.deploys + ' deployments</span>'
        + '<button class="btn btn-primary btn-sm" onclick="PlatformEngineering.deployTemplate(\'' + t.id + '\')">Deploy</button>'
        + '</div></div>';
    }).join('');
  }

  function renderTemplates() {
    var container = document.getElementById('pe-templates-grid');
    if (!container) return;
    container.innerHTML = goldenTemplates.map(function(t) {
      return '<div class="panel" style="padding:var(--space-md);border-left:3px solid #10b981;">'
        + '<div style="display:flex;align-items:center;gap:8px;margin-bottom:8px;">'
        + '<strong>' + t.name + '</strong>'
        + '<span style="font-size:10px;background:#10b981;color:#fff;padding:1px 6px;border-radius:3px;">✓ Approved</span>'
        + '</div>'
        + '<div style="font-size:12px;color:var(--color-muted);margin-bottom:8px;">Environment: <strong>' + t.env + '</strong></div>'
        + '<div style="font-size:12px;margin-bottom:4px;">CPU: ' + t.resources.cpu + ' · Memory: ' + t.resources.memory + '</div>'
        + '<div style="display:flex;flex-wrap:wrap;gap:4px;">' + t.features.map(function(f){ return '<span style="font-size:10px;background:var(--color-surface);border:1px solid var(--color-hairline);padding:2px 8px;border-radius:4px;">' + f + '</span>'; }).join('') + '</div>'
        + '</div>';
    }).join('');
  }

  function renderBlueprints() {
    var container = document.getElementById('pe-blueprints-grid');
    if (!container) return;
    container.innerHTML = blueprints.map(function(bp) {
      return '<div class="panel" style="padding:var(--space-md);">'
        + '<h4 style="margin:0 0 var(--space-sm);font-size:14px;">🏗️ ' + bp.name + '</h4>'
        + '<div style="margin-bottom:8px;"><span style="font-size:11px;font-weight:600;color:var(--color-muted);">SERVICES</span>'
        + '<div style="display:flex;flex-wrap:wrap;gap:4px;margin-top:4px;">' + bp.services.map(function(s){ return '<span style="font-size:11px;background:#6366f120;color:#6366f1;padding:2px 8px;border-radius:4px;">' + s + '</span>'; }).join('') + '</div></div>'
        + '<div><span style="font-size:11px;font-weight:600;color:var(--color-muted);">INFRASTRUCTURE</span>'
        + '<div style="display:flex;flex-wrap:wrap;gap:4px;margin-top:4px;">' + bp.infra.map(function(i){ return '<span style="font-size:11px;background:#f9731620;color:#f97316;padding:2px 8px;border-radius:4px;">' + i + '</span>'; }).join('') + '</div></div>'
        + '<button class="btn btn-ghost btn-sm" style="margin-top:var(--space-sm);" onclick="alert(\'Blueprint deployment wizard would launch here.\')">Deploy Blueprint</button>'
        + '</div>';
    }).join('');
  }

  global.PlatformEngineering = {
    init: function() { UIComponents.initTabs('pe-tab-btn', 'pe-tab-panel', 'data-pe-tab'); this.refresh(); },
    refresh: function() { renderCatalog(); renderTemplates(); renderBlueprints(); },
    deployTemplate: function(id) {
      var tpl = catalog.find(function(t){ return t.id === id; });
      if (!tpl || !global.Modal) return;
      global.Modal.open({
        title: tpl.icon + ' Deploy: ' + tpl.name,
        body: '<div style="padding:var(--space-xs);">'
          + '<div class="form-group"><label class="form-label">Service Name</label><input type="text" class="form-select" placeholder="my-service"></div>'
          + '<div style="display:grid;grid-template-columns:1fr 1fr;gap:var(--space-sm);">'
          + '<div class="form-group"><label class="form-label">Namespace</label><select class="form-select"><option>production</option><option>staging</option><option>development</option></select></div>'
          + '<div class="form-group"><label class="form-label">Replicas</label><input type="number" class="form-select" value="3"></div></div>'
          + '<div class="form-group"><label class="form-label">Image</label><input type="text" class="form-select" placeholder="registry.example.com/my-service:latest"></div>'
          + '<button class="btn btn-primary btn-sm" onclick="alert(\'Deploying from template...\');Modal.close();">Deploy</button>'
          + '</div>'
      });
    }
  };
})(window);
