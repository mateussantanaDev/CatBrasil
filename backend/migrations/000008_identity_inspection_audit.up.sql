ALTER TABLE identity_events DROP CONSTRAINT IF EXISTS identity_events_action_check;
ALTER TABLE identity_events ADD CONSTRAINT identity_events_action_check
    CHECK (action IN ('submitted', 'verified', 'rejected', 'resubmitted', 'inspected'));
