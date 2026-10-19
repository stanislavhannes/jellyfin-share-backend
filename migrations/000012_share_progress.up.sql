-- Where a link's viewers left off: one row per item the link contains - the
-- film, or each episode. It belongs to the link, so whoever holds it resumes on
-- any device; it goes when the link is deleted.
CREATE TABLE share_progress (
    share_id      UUID NOT NULL REFERENCES shares(id) ON DELETE CASCADE,
    item_id       VARCHAR(64) NOT NULL,
    position_secs INTEGER NOT NULL,
    duration_secs INTEGER NOT NULL,
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (share_id, item_id)
);
