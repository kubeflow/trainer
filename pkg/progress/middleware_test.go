package progress

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"k8s.io/klog/v2/ktesting"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestRecoveryMiddleware(t *testing.T) {
	logger, _ := ktesting.NewTestContext(t)
	ctrl.SetLogger(logger)

	// Create a handler that panics
	panicHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	})

	// Wrap with recovery middleware
	handler := recoveryMiddleware(logger)(panicHandler)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	// Should not panic, should return 500
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %v, want %v", rec.Code, http.StatusInternalServerError)
	}
}
