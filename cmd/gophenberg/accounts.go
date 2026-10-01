// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"time"

	"github.com/gopherium/framework/gonsole"
	accounts "github.com/gopherium/framework/gonsole/auth"

	"github.com/gopherium/gophenberg/internal/role"
)

// recordTimeout bounds storing one command record when COMMAND_RECORD_TIMEOUT names no other.
const recordTimeout = 5 * time.Second

// recordsLimit is how many records account:records lists when COMMAND_RECORDS_LIMIT names no other.
const recordsLimit = 50

// accountConfig returns the configuration of the account commands and of the check on the acting account.
func accountConfig() accounts.Config {
	return accounts.Config{
		Roles:         fixedRoles,
		Capability:    string(role.ManageUsers),
		RecordTimeout: recordTimeout,
		RecordsLimit:  recordsLimit,
	}
}

// fixedRoles answers the role table every Gophenberg account takes its role from.
func fixedRoles(context.Context, gonsole.Call) (accounts.Roles, error) {
	known := role.Known()
	capabilities := make(map[string][]string, len(known))
	for _, name := range known {
		capabilities[name] = role.CapabilitiesOf(name)
	}
	return accounts.Roles{Known: known, Privileged: role.Privileged(), Capabilities: capabilities}, nil
}
