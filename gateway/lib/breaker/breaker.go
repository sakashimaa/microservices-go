package breaker

import (
	"time"

	"github.com/sony/gobreaker"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func NewStandardCircuitBreaker(name string) *gobreaker.CircuitBreaker {
	return gobreaker.NewCircuitBreaker(gobreaker.Settings{
		Name:        name,
		MaxRequests: 1,
		Interval:    0,
		Timeout:     60 * time.Second,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			return counts.ConsecutiveFailures >= 5
		},
		IsSuccessful: func(err error) bool {
			if err == nil {
				return true
			}

			st, ok := status.FromError(err)
			if !ok {
				return false
			}

			switch st.Code() {
			case codes.InvalidArgument,
				codes.NotFound,
				codes.AlreadyExists,
				codes.Unauthenticated,
				codes.PermissionDenied,
				codes.FailedPrecondition:
				return true
			default:
				return false
			}
		},
	})
}
