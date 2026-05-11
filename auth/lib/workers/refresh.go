package workers

import (
	"context"
	"fmt"
	"time"

	"github.com/sakashimaa/billing-microservice/auth/repository"
)

func StartCleanupWorker(ctx context.Context, repo repository.AuthRepository, limit int) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			fmt.Printf("worker waked up, cleaning %d rows now\n", limit)

			err := repo.DeleteExpiredTokens(ctx, limit)
			if err != nil {
				fmt.Printf("failed to delete batch of expired tokens: %v\n", err)
			}
		case <-ctx.Done():
			fmt.Println("cleanup worker stopped gracefully")
			return
		}
	}
}
