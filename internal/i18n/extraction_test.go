// SPDX-License-Identifier: Apache-2.0

package i18n_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"text/template"
	"text/template/parse"
)

// found is one message and the file that declares it.
type found struct {
	msgid string
	file  string
}

// goRoots holds the directories the Go side's messages live under, from this package.
var goRoots = []string{"../../internal", "../../cmd", "../../plugins"}

// potMsgids returns every message the committed catalogue template carries.
func potMsgids(t *testing.T) map[string]bool {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "languages", "gophenberg.pot"))
	if err != nil {
		t.Fatal(err)
	}
	held := map[string]bool{}
	for _, line := range strings.Split(string(raw), "\n") {
		quoted, cut := strings.CutPrefix(line, "msgid ")
		if !cut {
			continue
		}
		msgid, err := strconv.Unquote(quoted)
		if err != nil {
			t.Fatalf("the template holds an unreadable line %q: %v", line, err)
		}
		held[msgid] = true
	}
	return held
}

// sourceFiles returns every file with the given ending under the Go side's roots.
func sourceFiles(t *testing.T, ending string) []string {
	t.Helper()
	var held []string
	for _, root := range goRoots {
		err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() && (entry.Name() == "node_modules" || entry.Name() == "dist") {
				return filepath.SkipDir
			}
			if !entry.IsDir() && strings.HasSuffix(path, ending) && !strings.HasSuffix(path, "_test.go") {
				held = append(held, path)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	return held
}

// markerName returns the name a call expression calls, empty for an unnamed one.
func markerName(call *ast.CallExpr) string {
	if named, ok := call.Fun.(*ast.Ident); ok {
		return named.Name
	}
	if selected, ok := call.Fun.(*ast.SelectorExpr); ok {
		return selected.Sel.Name
	}
	return ""
}

// goMarkers returns every message the Go source marks for extraction.
func goMarkers(t *testing.T) []found {
	t.Helper()
	var held []found
	for _, file := range sourceFiles(t, ".go") {
		parsed, err := parser.ParseFile(token.NewFileSet(), file, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(parsed, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok || markerName(call) != "Msgid" || len(call.Args) != 1 {
				return true
			}
			lit, ok := call.Args[0].(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				t.Errorf("%s marks a message that is not one quoted string", file)
				return true
			}
			msgid, err := strconv.Unquote(lit.Value)
			if err != nil {
				t.Errorf("%s marks an unreadable string %s", file, lit.Value)
				return true
			}
			held = append(held, found{msgid: msgid, file: file})
			return true
		})
	}
	return held
}

// askedOf returns the message a template command asks the translator for, if it asks one.
func askedOf(command *parse.CommandNode) (string, bool) {
	if len(command.Args) < 2 {
		return "", false
	}
	idents := identsOf(command.Args[0])
	if len(idents) < 2 || idents[len(idents)-1] != "Get" || idents[len(idents)-2] != "T" {
		return "", false
	}
	quoted, ok := command.Args[1].(*parse.StringNode)
	if !ok {
		return "", false
	}
	return quoted.Text, true
}

// identsOf returns the field chain a template argument walks, empty for other shapes.
func identsOf(arg parse.Node) []string {
	if field, ok := arg.(*parse.FieldNode); ok {
		return field.Ident
	}
	if variable, ok := arg.(*parse.VariableNode); ok {
		return variable.Ident
	}
	return nil
}

// walkNodes visits every command in a template tree.
func walkNodes(node parse.Node, visit func(*parse.CommandNode)) {
	switch held := node.(type) {
	case *parse.ListNode:
		for _, child := range held.Nodes {
			walkNodes(child, visit)
		}
	case *parse.ActionNode:
		walkPipe(held.Pipe, visit)
	case *parse.IfNode:
		walkBranch(held.BranchNode, visit)
	case *parse.RangeNode:
		walkBranch(held.BranchNode, visit)
	case *parse.WithNode:
		walkBranch(held.BranchNode, visit)
	case *parse.TemplateNode:
		walkPipe(held.Pipe, visit)
	}
}

// walkBranch visits every command a branching node holds.
func walkBranch(held parse.BranchNode, visit func(*parse.CommandNode)) {
	walkPipe(held.Pipe, visit)
	walkNodes(held.List, visit)
	if held.ElseList != nil {
		walkNodes(held.ElseList, visit)
	}
}

// walkPipe visits every command a pipeline holds.
func walkPipe(pipe *parse.PipeNode, visit func(*parse.CommandNode)) {
	if pipe == nil {
		return
	}
	for _, command := range pipe.Cmds {
		visit(command)
	}
}

// templateAsks returns every message the Go templates ask the translator for.
func templateAsks(t *testing.T) []found {
	t.Helper()
	var held []found
	for _, file := range sourceFiles(t, ".html") {
		raw, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		parsed, err := template.New(filepath.Base(file)).Parse(string(raw))
		if err != nil {
			t.Fatal(err)
		}
		for _, defined := range parsed.Templates() {
			if defined.Tree == nil || defined.Root == nil {
				continue
			}
			walkNodes(defined.Root, func(command *parse.CommandNode) {
				if msgid, ok := askedOf(command); ok {
					held = append(held, found{msgid: msgid, file: file})
				}
			})
		}
	}
	return held
}

func TestTheParsersFindTheMessagesTheExtractionMustFind(t *testing.T) {
	t.Parallel()

	markers := goMarkers(t)
	asks := templateAsks(t)

	if len(markers) < 14 {
		t.Fatalf("found %d marked messages, the walker lost its footing", len(markers))
	}
	if len(asks) < 9 {
		t.Fatalf("found %d template messages, the walker lost its footing", len(asks))
	}
}

func TestEveryGoSideMessageReachesTheCatalogueTemplate(t *testing.T) {
	t.Parallel()

	held := potMsgids(t)
	for _, message := range append(goMarkers(t), templateAsks(t)...) {
		if !held[message.msgid] {
			t.Errorf("%s declares %q, which the catalogue template does not carry", message.file, message.msgid)
		}
	}
}
