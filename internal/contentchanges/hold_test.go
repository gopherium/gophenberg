// SPDX-License-Identifier: Apache-2.0

package contentchanges_test

import (
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestHoldKeepsTheWritesOfOneItemInOrder(t *testing.T) {
	t.Parallel()

	n := notifier(&listeners{}, &shaper{})
	item := uuid.Must(uuid.NewV7())
	var mu sync.Mutex
	var order []string
	release := n.Hold(item)
	second := make(chan struct{})
	go func() {
		defer close(second)
		release := n.Hold(item)
		defer release()
		mu.Lock()
		order = append(order, "second")
		mu.Unlock()
	}()
	other := n.Hold(uuid.Must(uuid.NewV7()))
	other()
	time.Sleep(10 * time.Millisecond)
	mu.Lock()
	order = append(order, "first")
	mu.Unlock()
	release()
	<-second

	if !slices.Equal(order, []string{"first", "second"}) {
		t.Errorf("the writes ran in the order %v, want the second to wait for the first", order)
	}
}

func TestHoldLocksNothingWhenNoPluginListens(t *testing.T) {
	t.Parallel()

	n := notifier(&listeners{deaf: true}, &shaper{})
	item := uuid.Must(uuid.NewV7())

	first := n.Hold(item)
	second := n.Hold(item)
	second()
	first()
}
