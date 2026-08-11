window.ChangeManagement = createDataTablePage({
  idPrefix: 'change',
  containerId: 'change-management',
  title: 'Change Management',
  description: 'Enterprise governance and deployment approvals',
  viewType: 'list',
  columns: ['ID', 'Title', 'Type', 'Target', 'Requester', 'Scheduled', 'Actions'],
  endpoint: '/changes',
  renderRow: function(m) { return '<tr><td>' + m.id + '</td><td>' + m.title + '</td><td>' + m.type + '</td><td>' + m.target + '</td><td>' + m.requester + '</td><td>' + m.scheduled_for + '</td><td><button>Review</button></td></tr>'; }
});