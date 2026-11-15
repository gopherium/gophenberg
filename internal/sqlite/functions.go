// SPDX-License-Identifier: Apache-2.0

package sqlite

import "github.com/gopherium/framework/dbkit"

// functions lists the Go functions every SQLite handle of the site registers, built once per process.
var functions = mustFunctions(dbkit.NewFunctionList(
	dbkit.CaseFold(),
	dbkit.Function{Name: "strip_field_path", Args: 2, Deterministic: true, Call: stripFieldPath},
	dbkit.Function{Name: "strip_layout", Args: 2, Deterministic: true, Call: stripLayout},
	dbkit.Function{Name: "fields_contain", Args: 2, Deterministic: true, Call: fieldsContain},
))

// mustFunctions returns list and panics with err when dbkit could not build it.
func mustFunctions(list *dbkit.FunctionList, err error) *dbkit.FunctionList {
	if err != nil {
		panic(err)
	}
	return list
}
