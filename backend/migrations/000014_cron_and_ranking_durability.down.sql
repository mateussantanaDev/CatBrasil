DROP TABLE IF EXISTS cron_executions;
ALTER TABLE mining_race_weekly DROP CONSTRAINT IF EXISTS mining_race_weekly_pkey;
DROP INDEX IF EXISTS idx_mining_race_period_tonnage;
ALTER TABLE mining_race_weekly DROP COLUMN IF EXISTS period_start;
ALTER TABLE mining_race_weekly ADD PRIMARY KEY (user_id);
DROP INDEX IF EXISTS idx_users_last_spin_date;
ALTER TABLE users DROP COLUMN IF EXISTS last_spin_date;
ALTER TABLE users DROP COLUMN IF EXISTS last_spin_at;
