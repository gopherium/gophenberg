// SPDX-License-Identifier: Apache-2.0

package server

import (
	"context"
	"errors"
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/gopherium/gouncer/authkit"

	"github.com/gopherium/gophenberg/internal/update"
	"github.com/gopherium/gophenberg/internal/updater"
)

// ErrUpdateRefused marks the start of an update that a check before its download refused, its message saying why.
var ErrUpdateRefused = errors.New("server: the start of the update was refused")

// UpdateChecks is what the update routes ask of the site: its overview, what the checks stored, a check and a start.
type UpdateChecks interface {
	Overview(ctx context.Context) (UpdatesOverview, error)
	Result(ctx context.Context) (update.Result, error)
	Check(ctx context.Context) (update.Result, error)
	Start(ctx context.Context, asked UpdateStart) error
}

// UpdateStart is what an administrator asks of the start of an update: the version to update to and whether to retry.
type UpdateStart struct {
	Version string `json:"version"`
	Retry   bool   `json:"retry"`
}

// UpdatesScreen is how often the Updates screen asks for the progress of an update and how long it waits for a restart.
type UpdatesScreen struct {
	ProgressInterval time.Duration
	RestartWait      time.Duration
}

// UpdatesOverview is what GET /api/updates answers: what the checks found, this binary, its install and the next check.
type UpdatesOverview struct {
	update.Result
	updater.Overview
	Running           string         `json:"running"`
	RunningPostgreSQL *int           `json:"running_postgresql,omitempty"`
	CanUpdate         bool           `json:"can_update"`
	CannotUpdate      updater.Reason `json:"cannot_update,omitempty"`
	NextCheck         *time.Time     `json:"next_check"`
}

// handleUpdatesGet answers the overview of the updates: what the checks stored beside this binary and its install.
func (s *server) handleUpdatesGet(updates UpdateChecks) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		overview, err := updates.Overview(r.Context())
		if err != nil {
			respondDomainError(w, err)
			return
		}
		authkit.Respond(w, http.StatusOK, overview)
	}
}

// handleUpdatesApply starts the update an administrator asks for and answers the overview while the rest runs.
func (s *server) handleUpdatesApply(updates UpdateChecks) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		asked, err := decodeKnown[UpdateStart](w, r)
		if err != nil {
			respondBodyError(w, err)
			return
		}
		if asked.Version == "" {
			authkit.RespondError(w, http.StatusUnprocessableEntity, authkit.ErrorResponse{
				Message: "the start of an update needs the version to update to", Code: "version_required",
			})
			return
		}
		if err := updates.Start(r.Context(), asked); err != nil {
			respondDomainError(w, err)
			return
		}
		overview, err := updates.Overview(r.Context())
		if err != nil {
			respondDomainError(w, err)
			return
		}
		authkit.Respond(w, http.StatusAccepted, overview)
	}
}

// handleUpdatesCheck runs a check now and answers its result, refusing one within age of the last.
func (s *server) handleUpdatesCheck(updates UpdateChecks, age time.Duration) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		last, err := updates.Result(r.Context())
		if err != nil {
			respondDomainError(w, err)
			return
		}
		if wait := age - time.Since(lastCheck(last)); wait > 0 {
			seconds := int(math.Ceil(wait.Seconds()))
			w.Header().Set("Retry-After", strconv.Itoa(seconds))
			authkit.RespondError(w, http.StatusTooManyRequests, authkit.ErrorResponse{
				Message: "the last check is too recent, so try again later", Code: "check_too_soon",
				Meta: map[string]any{"retry_after": seconds},
			})
			return
		}
		result, err := updates.Check(r.Context())
		if unanswerable(result, err) {
			respondDomainError(w, err)
			return
		}
		authkit.Respond(w, http.StatusOK, result)
	}
}

// unanswerable reports whether a check's failure leaves no stored result to answer with.
func unanswerable(result update.Result, err error) bool {
	if errors.Is(err, update.ErrNotStored) {
		return true
	}
	return err != nil && result.FailedAt == nil
}

// lastCheck returns when the last check ended, a success or a failure, the zero time before any.
func lastCheck(result update.Result) time.Time {
	var last time.Time
	for _, at := range []*time.Time{result.CheckedAt, result.FailedAt} {
		if at != nil && at.After(last) {
			last = *at
		}
	}
	return last
}
