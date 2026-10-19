package database

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jellyfin-share/jellyfin-share-backend/internal/models"
)

// SaveProgress records where a link's viewers got to in one item, whichever
// device sent it. The WHERE clause is models.ReplacesProgress, done inside the
// upsert so two devices reporting at once cannot slip past it: a report under
// the floor does not replace a position at or over it.
func (db *DB) SaveProgress(ctx context.Context, shareID uuid.UUID, itemID string, positionSecs, durationSecs int64) error {
	query := `
		INSERT INTO share_progress (share_id, item_id, position_secs, duration_secs, updated_at)
		VALUES ($1, $2, $3, $4, NOW())
		ON CONFLICT (share_id, item_id) DO UPDATE
		SET position_secs = EXCLUDED.position_secs,
		    duration_secs = EXCLUDED.duration_secs,
		    updated_at = NOW()
		WHERE EXCLUDED.position_secs >= $5 OR share_progress.position_secs < $5`
	if _, err := db.ExecContext(ctx, query, shareID, itemID, positionSecs, durationSecs, models.MinResumeSecs); err != nil {
		return fmt.Errorf("failed to save progress: %w", err)
	}
	return nil
}

// GetProgress returns a link's progress for every item watched, newest first.
func (db *DB) GetProgress(ctx context.Context, shareID uuid.UUID) ([]models.ItemProgress, error) {
	var rows []models.ItemProgress
	query := `
		SELECT item_id, position_secs, duration_secs, updated_at
		FROM share_progress WHERE share_id = $1
		ORDER BY updated_at DESC`
	if err := db.SelectContext(ctx, &rows, query, shareID); err != nil {
		return nil, fmt.Errorf("failed to get progress: %w", err)
	}
	return rows, nil
}
