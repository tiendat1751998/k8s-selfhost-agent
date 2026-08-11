-- Migration: 008_deployment_promotion.sql
-- Creates tables for deployment promotion.

CREATE TABLE IF NOT EXISTS promotions (
    id            UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    service       VARCHAR(255) NOT NULL,
    version       VARCHAR(255) NOT NULL,
    from_env      VARCHAR(50)  NOT NULL, -- dev | qa | staging
    to_env        VARCHAR(50)  NOT NULL, -- qa | staging | production
    status        VARCHAR(50)  NOT NULL DEFAULT 'pending', -- pending | approved | promoting | completed | failed
    requester     VARCHAR(255) NOT NULL,
    approver      VARCHAR(255),
    approved_at   TIMESTAMPTZ,
    completed_at  TIMESTAMPTZ,
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_promo_service ON promotions (service);
CREATE INDEX idx_promo_status ON promotions (status);
CREATE INDEX idx_promo_created ON promotions (created_at DESC);
