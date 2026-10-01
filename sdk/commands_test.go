// SPDX-License-Identifier: Apache-2.0

package sdk

import (
	"errors"
	"testing"

	"github.com/gopherium/framework/gonsole"
)

func TestMisuseMarksAMisusedCommandLine(t *testing.T) {
	t.Parallel()

	err := Misuse(errors.New("feed:refresh takes no arguments"))

	if !errors.Is(err, gonsole.ErrMisused) {
		t.Errorf("Misuse() = %v, want it marked as a misused command line", err)
	}
}

func TestEnvReadsAPluginSettingUnderTheProgramPrefix(t *testing.T) {
	t.Parallel()

	env := Env{Prefix: "GOPHENBERG_", Getenv: func(key string) string {
		return map[string]string{"GOPHENBERG_FEED_ITEMS": " 7 "}[key]
	}}

	items, err := env.Within("FEED_").Count("ITEMS", 20)

	if err != nil || items != 7 {
		t.Errorf("Count(ITEMS) = %d, %v, want 7 read trimmed under the program prefix", items, err)
	}
}
