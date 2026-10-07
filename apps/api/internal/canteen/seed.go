package canteen

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"jiaohao/internal/logx"
)

func SeedDemo(ctx context.Context, store *Store, staffStudentID string, log *logx.Logger) error {
	n, err := store.CountCanteens(ctx)
	if err != nil {
		return err
	}
	if n > 0 {
		return nil
	}

	c1, err := store.CreateCanteen(ctx, "第一食堂", 1)
	if err != nil {
		return fmt.Errorf("seed canteen 1: %w", err)
	}
	c2, err := store.CreateCanteen(ctx, "第二食堂", 2)
	if err != nil {
		return fmt.Errorf("seed canteen 2: %w", err)
	}
	f1, err := store.CreateFloor(ctx, c1.ID, "1F", 1)
	if err != nil {
		return fmt.Errorf("seed floor 1: %w", err)
	}
	f2, err := store.CreateFloor(ctx, c1.ID, "2F", 2)
	if err != nil {
		return fmt.Errorf("seed floor 2: %w", err)
	}

	w1, err := store.CreateWindow(ctx, c1.ID, &f1.ID, "一号窗口", "W1", "盖饭", 1)
	if err != nil {
		return fmt.Errorf("seed window 1: %w", err)
	}
	if _, err := store.CreateWindow(ctx, c1.ID, &f2.ID, "二号窗口", "W2", "面食", 2); err != nil {
		return fmt.Errorf("seed window 2: %w", err)
	}
	if _, err := store.CreateWindow(ctx, c2.ID, (*uuid.UUID)(nil), "三号窗口", "W3", "快餐", 1); err != nil {
		return fmt.Errorf("seed window 3: %w", err)
	}
	if err := store.GrantStaffWindow(ctx, staffStudentID, w1); err != nil {
		log.Warn("canteen.seed", map[string]any{
			"outcome": "staff_grant_skipped",
			"error":   err.Error(),
		})
	}
	log.Info("canteen.seed", map[string]any{
		"outcome":     "created",
		"canteen_ids": []string{c1.ID.String(), c2.ID.String()},
	})
	return nil
}
