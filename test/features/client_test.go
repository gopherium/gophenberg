// SPDX-License-Identifier: Apache-2.0

package features_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
)

// answer is what the server replied to the last request a step made.
type answer struct {
	status int
	body   []byte
	header http.Header
}

// errorMessage returns the message a JSON error response carries.
func (a *answer) errorMessage() string {
	var envelope struct {
		Error string `json:"error"`
	}
	_ = json.Unmarshal(a.body, &envelope)
	return envelope.Error
}

// decode reads the answer's body into target.
func (a *answer) decode(target any) error {
	if err := json.Unmarshal(a.body, target); err != nil {
		return fmt.Errorf("reading the answer %q: %w", a.body, err)
	}
	return nil
}

// send asks the running server and returns what came back, leaving the last answer alone.
func (w *world) send(method, path, contentType string, body io.Reader) (*answer, error) {
	if err := w.running(); err != nil {
		return nil, err
	}
	request, err := http.NewRequest(method, w.site.URL+path, body)
	if err != nil {
		return nil, fmt.Errorf("building the %s %s request: %w", method, path, err)
	}
	if contentType != "" {
		request.Header.Set("Content-Type", contentType)
	}
	response, err := w.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("sending %s %s: %w", method, path, err)
	}
	defer func() { _ = response.Body.Close() }()

	read, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("reading the answer to %s %s: %w", method, path, err)
	}
	return &answer{status: response.StatusCode, body: read, header: response.Header}, nil
}

// do sends a request to the running server and records what came back.
func (w *world) do(method, path, contentType string, body io.Reader) error {
	got, err := w.send(method, path, contentType, body)
	if err != nil {
		return err
	}
	w.answer = got
	return nil
}

// postJSON sends a JSON body to the running server.
func (w *world) postJSON(path, body string) error {
	return w.do(http.MethodPost, path, "application/json", strings.NewReader(body))
}

// get asks the running server for a path.
func (w *world) get(path string) error {
	return w.do(http.MethodGet, path, "", nil)
}

// upload posts a payload as a multipart file under the given field.
func (w *world) upload(path, field, filename string, payload []byte) error {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile(field, filename)
	if err != nil {
		return fmt.Errorf("building the upload: %w", err)
	}
	if _, err := part.Write(payload); err != nil {
		return fmt.Errorf("writing the upload: %w", err)
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("closing the upload: %w", err)
	}
	return w.do(http.MethodPost, path, writer.FormDataContentType(), &body)
}

// activation is a request still in flight that switches the theme.
type activation struct {
	done   chan struct{}
	answer *answer
	err    error
}

// beginActivation asks the server to switch themes without waiting for the switch to finish.
func (w *world) beginActivation(method, path, body string) error {
	if err := w.running(); err != nil {
		return err
	}
	pending := &activation{done: make(chan struct{})}
	w.pending = pending
	go func() {
		defer close(pending.done)
		pending.answer, pending.err = w.send(method, path, "application/json", strings.NewReader(body))
	}()
	return nil
}

// settle waits for a switch in flight and records its answer as the last one.
func (w *world) settle() error {
	if w.pending == nil {
		return nil
	}
	pending := w.pending
	w.pending = nil
	<-pending.done
	if pending.err != nil {
		return pending.err
	}
	w.answer = pending.answer
	return nil
}

// expect returns the error that the last answer did not carry the wanted status.
func (w *world) expect(status int) error {
	if w.answer == nil {
		return fmt.Errorf("no request was made, want status %d", status)
	}
	if w.answer.status != status {
		return fmt.Errorf("status = %d (%s), want %d", w.answer.status, w.answer.body, status)
	}
	return nil
}
