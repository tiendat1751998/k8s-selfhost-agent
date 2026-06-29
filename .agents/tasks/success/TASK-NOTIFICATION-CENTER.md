# TASK: Notification Center

## Priority: HIGH

## Objective
Build a multi-channel notification center with configurable alert routing.

## Requirements

### Notification Channels
- Email configuration (SMTP settings)
- Slack integration (webhook URL, channel)
- Microsoft Teams (webhook URL)
- Telegram (bot token, chat ID)

### Notification Rules
- Route alerts by severity (critical → Slack + Email, warning → Slack)
- Route by resource type (deployment, pod, node)
- Route by namespace/cluster
- Quiet hours / maintenance windows

### Notification History
- List of sent notifications
- Delivery status (sent, failed, pending)
- Filter by channel, severity, time range
- Retry failed notifications

### Bell Icon & Inbox
- Top-bar bell icon with unread count
- Notification inbox drawer
- Mark as read/unread
- Dismiss notifications

## Output
- New section: `#notifications`
- New module: `/modules/notifications/notification-center.js`
- Bell icon component in top-bar
- Sidebar entry under "Platform" group

## Verification
- Navigate to `#notifications`
- Configure a Slack channel
- View notification history
- Bell icon shows unread count
- Inbox drawer opens and lists notifications
