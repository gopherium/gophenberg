// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"

	"github.com/gopherium/framework/gonsole"
	accounts "github.com/gopherium/framework/gonsole/auth"

	"github.com/gopherium/gophenberg/internal/role"
)

// fixedRoles answers the role table every Gophenberg account takes its role from.
func fixedRoles(context.Context, gonsole.Call) (accounts.Roles, error) {
	known := role.Known()
	capabilities := make(map[string][]string, len(known))
	for _, name := range known {
		capabilities[name] = role.CapabilitiesOf(name)
	}
	return accounts.Roles{Known: known, Privileged: role.Privileged(), Capabilities: capabilities}, nil
}
