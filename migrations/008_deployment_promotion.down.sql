-- Migration: 008_deployment_promotion.down.sql
-- Description: Drops deployment promotion tables.

DROP TABLE IF EXISTS promotions CASCADE;
