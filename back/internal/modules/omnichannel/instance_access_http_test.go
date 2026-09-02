package omnichannel

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWriteInstanceAccessErrorMapsConflicts(t *testing.T) {
	tests := []struct {
		name string
		err  error
		code string
	}{
		{name: "revision", err: ErrInstanceAccessRevisionConflict, code: "instance_access_revision_conflict"},
		{name: "last manager", err: ErrLastInstanceManager, code: "last_instance_manager"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequestWithContext(t.Context(), http.MethodPut, "/v1/omnichannel/tenant/whatsapp/instances/a/users", nil)
			recorder := httptest.NewRecorder()

			writeInstanceAccessError(recorder, request, test.err)

			if recorder.Code != http.StatusConflict || !strings.Contains(recorder.Body.String(), test.code) {
				t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
			}
		})
	}
}
