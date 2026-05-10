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
	default:
		log.Printf("grpc error: %v", st.Message())
		api.SendJSON(w, http.StatusInternalServerError, "error", st.Message())
	}
}
