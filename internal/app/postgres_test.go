// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"
	"fmt"
	"math"
	"runtime"
	"strings"
	"testing"

	"github.com/gopherium/framework/dbkit"
	"github.com/gopherium/framework/goncierge"
	"github.com/gopherium/framework/gonsole"
	"github.com/gopherium/framework/gonsole/testkit"
)

// keywordValueAddress is a keyword and value PostgreSQL address at a port no test reaches.
const keywordValueAddress = "host=localhost port=9 user=postgres dbname=postgres sslmode=disable connect_timeout=1"

func TestComposeCapsThePostgreSQLPoolAtTheConnectionSetting(t *testing.T) {
	t.Parallel()

	for name, asked := range map[string]struct {
		value string
		want  int32
	}{
		"unset":        {"", int32(max(4, runtime.NumCPU()))},
		"the lowest":   {"2", 2},
		"three":        {"3", 3},
		"the greatest": {"2147483647", math.MaxInt32},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			built, err := compose(composeConfig{
				databaseURL: unreachableDatabaseURL, fieldDepth: 4, getenv: testkit.Getenv(map[string]string{
					"GOPHENBERG_DATABASE_MAX_CONNS": asked.value, "GOPHENBERG_PLUGIN_DB_RESERVE": "1",
				}),
			}, noPlugins)
			if err != nil {
				t.Fatalf("compose() error = %v, want nil", err)
			}
			defer built.close()

			if got := built.pgHandle.Pool.Config().MaxConns; got != asked.want {
				t.Errorf("the pool holds at most %d connections, want %d", got, asked.want)
			}
		})
	}
}

func TestComposeNamesTheConnectionCapItRefusesOnPostgreSQL(t *testing.T) {
	t.Parallel()

	for _, value := range []string{"1", "2147483648", "many"} {
		t.Run(value, func(t *testing.T) {
			t.Parallel()

			_, err := compose(composeConfig{
				databaseURL: unreachableDatabaseURL, fieldDepth: 4,
				getenv: testkit.Getenv(map[string]string{"GOPHENBERG_DATABASE_MAX_CONNS": value}),
			}, noPlugins)

			if message := fmt.Sprint(err); !strings.HasPrefix(message, "GOPHENBERG_DATABASE_MAX_CONNS: ") ||
				!strings.HasSuffix(message, `got "`+value+`"`) {
				t.Errorf("compose() with GOPHENBERG_DATABASE_MAX_CONNS=%s = %v, want an error naming the setting "+
					"and quoting its value", value, err)
			}
		})
	}
}

// poolSettingValues are the pool settings pgx reads out of an address, each with a value above zero and a zero.
var poolSettingValues = map[string][]string{
	"pool_max_conns":                {"4", "0"},
	"pool_min_conns":                {"1", "0"},
	"pool_min_idle_conns":           {"1", "0"},
	"pool_max_conn_lifetime":        {"1h", "0s"},
	"pool_max_conn_idle_time":       {"1h", "0s"},
	"pool_health_check_period":      {"1m", "0s"},
	"pool_max_conn_lifetime_jitter": {"1m", "0s"},
	"pool_ping_timeout":             {"1s", "0s"},
}

// poolSettingCase is one address that carries pool settings and the settings it carries, in sorted order.
type poolSettingCase struct {
	address string
	keys    string
}

// poolSettingAddresses returns every pool setting at each of its values in a URL and a keyword and value address.
func poolSettingAddresses() map[string]poolSettingCase {
	cases := map[string]poolSettingCase{
		"two pool settings": {
			unreachableDatabaseURL + "&pool_max_conns=4&pool_health_check_period=1m",
			"pool_health_check_period and pool_max_conns",
		},
	}
	for key, values := range poolSettingValues {
		for _, value := range values {
			pair := key + "=" + value
			cases[pair+" in a URL"] = poolSettingCase{unreachableDatabaseURL + "&" + pair, key}
			cases[pair+" in a keyword and value address"] = poolSettingCase{keywordValueAddress + " " + pair, key}
		}
	}
	return cases
}

// poolSettingError is the error that names the pool settings keys an address carries.
func poolSettingError(keys string) string {
	return "GOPHENBERG_DATABASE_URL: remove " + keys + " from the address, GOPHENBERG_DATABASE_MAX_CONNS sizes the pool"
}

func TestComposeNamesEveryPoolSettingInTheAddressAndTheSettingThatSizesThePool(t *testing.T) {
	t.Parallel()

	for name, asked := range poolSettingAddresses() {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			_, err := compose(composeConfig{databaseURL: asked.address, fieldDepth: 4}, noPlugins)

			if want := poolSettingError(asked.keys); fmt.Sprint(err) != want {
				t.Errorf("compose() = %v, want %q", err, want)
			}
		})
	}
}

func TestCheckNamesEveryPoolSettingInTheAddress(t *testing.T) {
	t.Parallel()

	for key, values := range poolSettingValues {
		t.Run(key, func(t *testing.T) {
			t.Parallel()

			getenv := testkit.Getenv(map[string]string{
				"GOPHENBERG_DATABASE_URL": unreachableDatabaseURL + "&" + key + "=" + values[0],
			})

			got := testkit.Run(t, Program(getenv, noPlugins), "", "check")

			if want := "gophenberg: " + poolSettingError(key) + "\n"; got.Code != gonsole.ExitFailed || got.Stderr != want {
				t.Errorf("check = %d with stderr %q, want 1 and %q", got.Code, got.Stderr, want)
			}
		})
	}
}

func TestComposeNamesTheDatabaseAddressForEachFormDbkitRefuses(t *testing.T) {
	t.Parallel()

	for name, address := range map[string]string{
		"a bare @ in the password":      "postgres://postgres:gophen@berg@localhost:9/postgres?sslmode=disable",
		"a query key set twice":         unreachableDatabaseURL + "&sslmode=require",
		"an address pgx cannot parse":   "not a url \x00",
		"no address outside describing": "",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			_, err := compose(composeConfig{databaseURL: address, fieldDepth: 4}, noPlugins)

			if !errors.Is(err, dbkit.ErrAddress) || !strings.HasPrefix(err.Error(), "GOPHENBERG_DATABASE_URL: ") {
				t.Errorf("compose() = %v, want the address refused under GOPHENBERG_DATABASE_URL", err)
			}
		})
	}
}

func TestComposeKeepsAKeywordAndValueAddressOnPostgreSQL(t *testing.T) {
	t.Parallel()

	built, err := compose(composeConfig{databaseURL: keywordValueAddress, fieldDepth: 4}, noPlugins)

	if err != nil {
		t.Fatalf("compose() over a keyword and value address = %v, want nil", err)
	}
	built.close()
}

func TestADescribingCallRegistersThePluginsOverAnAddressDbkitRefuses(t *testing.T) {
	t.Parallel()

	for name, address := range map[string]string{
		"an address pgx cannot parse": "not a url \x00",
		"a pool cap":                  unreachableDatabaseURL + "&pool_max_conns=4",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			k := &keeper{}
			call := gonsole.Call{
				Env: settingsEnv(testkit.Getenv(map[string]string{"GOPHENBERG_DATABASE_URL": address})), Describe: true,
			}

			loaded, err := loadPlugins(binary{plugins: keeping(k)}, goncierge.New(), &sitePool{})(t.Context(), call)
			if err != nil {
				t.Fatalf("loadPlugins() while describing = %v, want the plugins registered", err)
			}
			t.Cleanup(func() { _ = loaded.Release(context.WithoutCancel(t.Context())) })
			_, execErr := k.deps.DB.Exec(t.Context(), "SELECT 1")

			if !errors.Is(execErr, errLaneUnopened) {
				t.Errorf("Exec() while describing = %v, want %v with no pool opened", execErr, errLaneUnopened)
			}
		})
	}
}

func TestCheckNamesEachPostgreSQLDatabaseSettingItRefuses(t *testing.T) {
	t.Parallel()

	for name, asked := range map[string]struct {
		env  map[string]string
		want string
	}{
		"a connection cap of one": {
			map[string]string{"GOPHENBERG_DATABASE_URL": unreachableDatabaseURL, "GOPHENBERG_DATABASE_MAX_CONNS": "1"},
			"gophenberg: GOPHENBERG_DATABASE_MAX_CONNS: ",
		},
		"a pool cap in the address": {
			map[string]string{"GOPHENBERG_DATABASE_URL": unreachableDatabaseURL + "&pool_max_conns=4"},
			"gophenberg: GOPHENBERG_DATABASE_URL: ",
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got := testkit.Run(t, Program(testkit.Getenv(asked.env), noPlugins), "", "check")

			if got.Code != gonsole.ExitFailed || !strings.HasPrefix(got.Stderr, asked.want) {
				t.Errorf("check = %d with stderr %q, want 1 and a line starting %q", got.Code, got.Stderr, asked.want)
			}
		})
	}
}
