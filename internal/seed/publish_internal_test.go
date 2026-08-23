// SPDX-License-Identifier: Apache-2.0

package seed

import (
	"errors"
	"testing"

	"github.com/gopherium/gophenberg/internal/content"
)

func TestMustPublishPanicsOnContentTheStatusMachineRefuses(t *testing.T) {
	t.Parallel()

	trashed := content.Content{Status: content.StatusTrash}

	defer func() {
		recovered := recover()
		if recovered == nil {
			t.Fatal("mustPublish() returned, want the trashed item refused by panic")
		}
		refused, isError := recovered.(error)
		if !isError || !errors.Is(refused, content.ErrInvalidTransition) {
			t.Errorf("mustPublish() panic = %v, want %v", recovered, content.ErrInvalidTransition)
		}
	}()
	mustPublish(&trashed)
}
