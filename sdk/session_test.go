// SPDX-License-Identifier: Apache-2.0

package sdk

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

func TestSessionFromAnswersWhatTheHostFiled(t *testing.T) {
	t.Parallel()

	id := uuid.New()
	filed := NewSession(id, "maria@example.com", "Maria Perez", "editor", []string{"change_others_work"})

	held, ok := SessionFrom(WithSession(t.Context(), filed))

	if !ok {
		t.Fatal("SessionFrom() found no session, want the one the host filed")
	}
	if held.ID != id || held.Email != "maria@example.com" || held.Name != "Maria Perez" {
		t.Errorf("session = %+v, want the identity the host filed", held)
	}
	if held.Role != "editor" {
		t.Errorf("role = %q, want %q", held.Role, "editor")
	}
}

func TestSessionFromReportsARequestCarryingNone(t *testing.T) {
	t.Parallel()

	held, ok := SessionFrom(context.Background())

	if ok {
		t.Errorf("SessionFrom() = %+v, true, want no session on a request the host left unfiled", held)
	}
	if held.Can("change_others_work") {
		t.Error("an unfiled session holds a capability, want it to hold none")
	}
}

func TestCanAnswersOnlyTheCapabilitiesFiled(t *testing.T) {
	t.Parallel()

	filed := NewSession(uuid.New(), "maria@example.com", "Maria Perez", "editor",
		[]string{"change_others_work"})

	if !filed.Can("change_others_work") {
		t.Error("Can() refused a capability the host filed, want it held")
	}
	if filed.Can("manage_users") {
		t.Error("Can() held a capability the host never filed, want it refused")
	}
}

func TestAPluginCannotWidenTheCapabilitiesItWasGiven(t *testing.T) {
	t.Parallel()

	given := []string{"change_others_work"}
	filed := NewSession(uuid.New(), "maria@example.com", "Maria Perez", "editor", given)

	given[0] = "manage_users"

	if filed.Can("manage_users") {
		t.Error("writing to the caller's slice widened the session, want the session to hold its own copy")
	}
}
