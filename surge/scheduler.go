package surge

import (
	"context"
	"time"
)

func (c *Client) runScheduler(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			for {
				count, err := c.backend.DispatchScheduledJobs(ctx, 100)
				if err != nil {
					break
				}

				if count < 100 {
					break
				}
			}
		}
	}
}
