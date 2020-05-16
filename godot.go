// Package godot checks if all top-level comments contain a period at the
// end of the last sentence if needed.
package godot

import (
	"go/ast"
	"go/token"
	"regexp"
	"strings"
)

const noPeriodMessage = "Top level comment should end in a period"

// Settings contains linter settings.
type Settings struct {
	// Check all top-level comments, not only declarations
	CheckAll bool
}

// Issue contains a description of linting error and a possible replacement.
type Issue struct {
	Pos         token.Position
	Message     string
	Replacement string
}

// position is an position inside a comment (might be multiline comment).
type position struct {
	line   int
	column int
}

var (
	// List of valid last characters.
	lastChars = []string{".", "?", "!"}

	// Special tags in comments like "nolint" or "build".
	tags = regexp.MustCompile("^[a-z]+:")

	// Special hashtags in comments like "#nosec".
	hashtags = regexp.MustCompile("^#[a-z]+ ")

	// URL at the end of the line.
	endURL = regexp.MustCompile(`[a-z]+://[^\s]+$`)
)

// Run runs this linter on the provided code.
func Run(file *ast.File, fset *token.FileSet, settings Settings) []Issue {
	issues := []Issue{}

	// Check all top-level comments
	if settings.CheckAll {
		for _, group := range file.Comments {
			if iss, ok := check(fset, group); !ok {
				issues = append(issues, iss)
			}
		}
		return issues
	}

	// Check only declaration comments
	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.GenDecl:
			if iss, ok := check(fset, d.Doc); !ok {
				issues = append(issues, iss)
			}
		case *ast.FuncDecl:
			if iss, ok := check(fset, d.Doc); !ok {
				issues = append(issues, iss)
			}
		}
	}
	return issues
}

func check(fset *token.FileSet, group *ast.CommentGroup) (iss Issue, ok bool) {
	if group == nil || len(group.List) == 0 {
		return Issue{}, true
	}

	// Check only top-level comments
	if fset.Position(group.Pos()).Column > 1 {
		return Issue{}, true
	}

	// Get last element from comment group - it can be either
	// last (or single) line for "//"-comment, or multiline string
	// for "/*"-comment
	last := group.List[len(group.List)-1]

	p, ok := checkComment(last.Text)
	if ok {
		return Issue{}, true
	}
	pos := fset.Position(last.Slash)
	pos.Line += p.line
	pos.Column = p.column
	iss = Issue{
		Pos:         pos,
		Message:     noPeriodMessage,
		Replacement: makeReplacement(last.Text, p),
	}
	return iss, false
}

func checkComment(comment string) (pos position, ok bool) {
	// Check last line of "//"-comment
	if strings.HasPrefix(comment, "//") {
		pos.column = len(comment)
		comment = strings.TrimPrefix(comment, "//")
		if checkLastChar(comment) {
			return position{}, true
		}
		return pos, false
	}

	// Skip cgo code blocks
	// TODO: Find a better way to detect cgo code
	if strings.Contains(comment, "#include") || strings.Contains(comment, "#define") {
		return position{}, true
	}

	// Check last non-empty line in multiline "/*"-comment block
	lines := strings.Split(comment, "\n")
	var i int
	for i = len(lines) - 1; i >= 0; i-- {
		if s := strings.TrimSpace(lines[i]); s == "*/" || s == "" {
			continue
		}
		break
	}
	pos.line = i
	comment = lines[i]
	comment = strings.TrimSuffix(comment, "*/")
	comment = strings.TrimRight(comment, " ")
	pos.column = len(comment) // last non-space char in comment line
	comment = strings.TrimPrefix(comment, "/*")

	if checkLastChar(comment) {
		return position{}, true
	}
	return pos, false
}

func checkLastChar(s string) bool {
	// Don't check comments starting with space indentation - they may
	// contain code examples, which shouldn't end with period
	if strings.HasPrefix(s, "  ") || strings.HasPrefix(s, " \t") || strings.HasPrefix(s, "\t") {
		return true
	}
	// Skip cgo export tags: https://golang.org/cmd/cgo/#hdr-C_references_to_Go
	if strings.HasPrefix(s, "export") {
		return true
	}
	s = strings.TrimSpace(s)
	if tags.MatchString(s) ||
		hashtags.MatchString(s) ||
		endURL.MatchString(s) ||
		strings.HasPrefix(s, "+build") {
		return true
	}
	// Don't check empty lines
	if s == "" {
		return true
	}
	for _, ch := range lastChars {
		if string(s[len(s)-1]) == ch {
			return true
		}
	}
	return false
}

// makeReplacement basically just inserts a period into comment on
// the given position.
func makeReplacement(s string, pos position) string {
	lines := strings.Split(s, "\n")
	if len(lines) < pos.line {
		// This should never happen
		return ""
	}
	line := []rune(lines[pos.line])
	if len(line) < pos.column {
		// This should never happen
		return ""
	}
	// Insert a period
	newline := append(
		line[:pos.column],
		append([]rune{'.'}, line[pos.column:]...)...,
	)
	return string(newline)
}
