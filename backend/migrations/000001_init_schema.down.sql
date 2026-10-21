-- ============================================================================
-- CATBRASIL - MIGRATION 000001 DOWN: DROP ALL TABLES & EXTENSIONS
-- ============================================================================

DROP TABLE IF EXISTS sms_verifications CASCADE;
DROP TABLE IF EXISTS security_logs CASCADE;
DROP TABLE IF EXISTS usage_logs CASCADE;
DROP TABLE IF EXISTS mining_race_weekly CASCADE;
DROP TABLE IF EXISTS user_promos CASCADE;
DROP TABLE IF EXISTS system_config CASCADE;
DROP TABLE IF EXISTS announcements CASCADE;
DROP TABLE IF EXISTS roulette_slices CASCADE;
DROP TABLE IF EXISTS transactions CASCADE;
DROP TABLE IF EXISTS referrals CASCADE;
DROP TABLE IF EXISTS machines CASCADE;
DROP TABLE IF EXISTS fleet_catalog CASCADE;
DROP TABLE IF EXISTS users CASCADE;
