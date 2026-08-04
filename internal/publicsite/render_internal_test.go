// SPDX-License-Identifier: Apache-2.0

package publicsite

import (
	"html/template"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRenderReportsAPageItCouldNotBuild(t *testing.T) {
	t.Parallel()

	broken := template.Must(template.New("shell").Parse(`{{.Missing.Field}}`))
	recorder := httptest.NewRecorder()

	(&site{}).render(recorder, broken, http.StatusOK, listData{})

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusInternalServerError)
	}
	if strings.Contains(recorder.Body.String(), "Missing") {
		t.Errorf("body = %q, want the template failure masked", recorder.Body.String())
	}
}
