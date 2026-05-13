package grpc

import (
	"log"
	"net/http"

	"github.com/sakashimaa/billing-microservice/pkg/utils/api"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func HandleGRPCError(w http.ResponseWriter, err error) {
	st, ok := status.FromError(err)
	if !ok {
		api.SendJSON(w, http.StatusInternalServerError, "error", "internal server error")
		return
	}

	switch st.Code() {
	case codes.InvalidArgument:
		api.SendJSON(w, http.StatusBadRequest, "error", st.Message())
	case codes.NotFound:
		api.SendJSON(w, http.StatusNotFound, "error", st.Message())
	case codes.FailedPrecondition:
		api.SendJSON(w, http.StatusBadRequest, "error", st.Message())
	case codes.PermissionDenied:
		api.SendJSON(w, http.StatusForbidden, "error", st.Message())
	case codes.Unauthenticated:
		api.SendJSON(w, http.StatusUnauthorized, "error", st.Message())
	case codes.ResourceExhausted:
		api.SendJSON(w, http.StatusTooManyRequests, "error", st.Message())
	case codes.AlreadyExists:
		api.SendJSON(w, http.StatusConflict, "error", st.Message())
	case codes.Unavailable:
		log.Printf("grpc unavailable: %v\n", st.Message())
		api.SendJSON(w, http.StatusServiceUnavailable, "error", "service is temporarily unavailable")
	case codes.DeadlineExceeded:
		log.Printf("grpc deadline exceeded: %v\n", st.Message())
		api.SendJSON(w, http.StatusGatewayTimeout, "error", "request timeout")
	case codes.Unimplemented:
		log.Printf("grpc unimplemented: %v\n", st.Message())
		api.SendJSON(w, http.StatusNotImplemented, "error", "method not implemented")
	default:
		log.Printf("grpc internal error [code=%v]: %v\n", st.Code(), st.Message())
		api.SendJSON(w, http.StatusInternalServerError, "error", "internal server error")
	}
}
