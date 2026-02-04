package health

import (
	"encoding/json"
	"net/http"

	"github.com/lewisedginton/general_purpose_chatbot/pkg/logger"
)

// HealthResponse represents the JSON response structure for HTTP health check endpoints.
// It includes the overall status and detailed information about individual checks.
type HealthResponse struct {
	Status  string                 `json:"status"`            // "healthy" | "unhealthy"
	Checks  map[string]CheckStatus `json:"checks,omitempty"`  // check name -> status
	Message string                 `json:"message,omitempty"` // optional message
}

// CheckStatus represents the status of an individual check in the HTTP response.
type CheckStatus struct {
	Status  string `json:"status"`            // "ok" | "error"
	Error   string `json:"error,omitempty"`   // error message if status is "error"
	Latency string `json:"latency,omitempty"` // latency in human-readable format
}

// LivenessHandler returns an HTTP handler for liveness checks.
// Returns 200 OK if the process is alive, 503 Service Unavailable if it should be restarted.
func (h *HealthChecker) LivenessHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		status, err := h.CheckLiveness(r.Context())
		h.writeHealthResponse(w, status, err)
	}
}

// ReadinessHandler returns an HTTP handler for readiness checks.
// Returns 200 OK if the service is ready for traffic, 503 Service Unavailable if not ready.
func (h *HealthChecker) ReadinessHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		status, err := h.CheckReadiness(r.Context())
		h.writeHealthResponse(w, status, err)
	}
}

// writeHealthResponse writes the health check response as JSON.
func (h *HealthChecker) writeHealthResponse(w http.ResponseWriter, status *HealthStatus, err error) {
	w.Header().Set("Content-Type", "application/json")

	response := HealthResponse{
		Checks: make(map[string]CheckStatus),
	}

	if status.Healthy {
		response.Status = "healthy"
		w.WriteHeader(http.StatusOK)
	} else {
		response.Status = "unhealthy"
		w.WriteHeader(http.StatusServiceUnavailable)
		if err != nil {
			response.Message = err.Error()
		}
	}

	// Add individual check results
	for _, checkResult := range status.Checks {
		checkStatus := CheckStatus{
			Latency: checkResult.Latency.String(),
		}

		if checkResult.Healthy {
			checkStatus.Status = "ok"
		} else {
			checkStatus.Status = "error"
			checkStatus.Error = checkResult.Error
		}

		response.Checks[checkResult.Name] = checkStatus
	}

	// Marshal and write response
	if err := json.NewEncoder(w).Encode(response); err != nil {
		if h.logger != nil {
			h.logger.Error("Failed to encode health response",
				logger.StringField("error", err.Error()),
			)
		}
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}
