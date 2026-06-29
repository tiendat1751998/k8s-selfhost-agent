-- Migration: 018_notification.sql
-- Creates tables for the notification system.

CREATE TABLE IF NOT EXISTS notification_channels (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    type        VARCHAR(50) NOT NULL,
    name        VARCHAR(255) NOT NULL,
    webhook_url TEXT,
    config      JSONB DEFAULT '{}'::jsonb,
    enabled     BOOLEAN NOT NULL DEFAULT true,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS notifications (
    id            UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    channel_id    UUID REFERENCES notification_channels(id) ON DELETE SET NULL,
    title         VARCHAR(500) NOT NULL,
    severity      VARCHAR(50) NOT NULL,
    message       TEXT NOT NULL,
    status        VARCHAR(50) NOT NULL DEFAULT 'pending',
    error_detail  TEXT,
    is_read       BOOLEAN NOT NULL DEFAULT false,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_notifications_status ON notifications (status);
CREATE INDEX idx_notifications_is_read ON notifications (is_read);
CREATE INDEX idx_notifications_created_at ON notifications (created_at DESC);
