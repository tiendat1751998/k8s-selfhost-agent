-- Migration: 018_notification.down.sql
-- Description: Drops notifications and notification_channels tables.

DROP TABLE IF EXISTS notifications CASCADE;
DROP TABLE IF EXISTS notification_channels CASCADE;
