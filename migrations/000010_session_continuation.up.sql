-- An episode started by autoplay continues a viewing that was already counted,
-- rather than costing another play. The depth bounds the chain: a share limited
-- to one play buys at most one pass through the series, not an endless loop.
ALTER TABLE share_sessions
    ADD COLUMN continuation_depth INTEGER NOT NULL DEFAULT 0;
