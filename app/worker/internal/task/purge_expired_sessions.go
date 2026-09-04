package task

import (
	"context"
	"fmt"
	"time"

	"github.com/zhimma/grove/internal/model"
	"github.com/zhimma/grove/pkg/database"
	"github.com/zhimma/grove/pkg/logger"
)

// purgeBatchSize keeps each DELETE short so a large backlog does not hold row
// locks for the whole run.
const purgeBatchSize = 1000

type purgeExpiredSessions struct {
	dbs database.Connections
	now func() time.Time
}

func newPurgeExpiredSessions(dbs database.Connections) *purgeExpiredSessions {
	return &purgeExpiredSessions{dbs: dbs, now: time.Now}
}

// Run deletes console sessions whose expiry has passed. An expired session
// cannot authenticate or refresh, so the row is dead weight rather than an
// audit record — revoked-but-unexpired sessions are deliberately left alone,
// because those still bound a login that the session list should show.
func (p *purgeExpiredSessions) Run(ctx context.Context) error {
	db := p.dbs.Default()
	if db == nil {
		return fmt.Errorf("purge expired sessions requires a default database")
	}

	cutoff := p.now()
	deleted := int64(0)
	for {
		if err := ctx.Err(); err != nil {
			return err
		}

		result := db.WithContext(ctx).
			Where("expires_at <= ?", cutoff).
			Limit(purgeBatchSize).
			Delete(&model.ConsoleSession{})
		if result.Error != nil {
			return fmt.Errorf("delete expired console sessions: %w", result.Error)
		}
		deleted += result.RowsAffected
		if result.RowsAffected < purgeBatchSize {
			break
		}
	}

	logger.Info().Int64("deleted", deleted).Msg("已清理过期后台会话")
	return nil
}
