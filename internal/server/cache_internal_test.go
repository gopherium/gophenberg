// SPDX-License-Identifier: Apache-2.0

package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCacheStampMarksAnAnswerWrittenWithoutAStatus(t *testing.T) {
	t.Parallel()

	handler := cacheStamped(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("served"))
	}), "public, max-age=1")
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))

	if held := recorder.Header().Get("Cache-Control"); held != "public, max-age=1" {
		t.Errorf("Cache-Control = %q, want an answer written straight away stamped as a success", held)
	}
}
