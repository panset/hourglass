// SPDX-License-Identifier: MIT

package settings

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"text/template"
	"text/template/parse"
)

const (
	// MaxTemplateLen is the longest resource-name template accepted (SET-4).
	MaxTemplateLen = 1024
	// MaxResourceNameLen is the longest rendered resource name; a template
	// that renders more fails.
	MaxResourceNameLen = 253
)

var errTooLong = fmt.Errorf("rendered resource name is longer than %d characters", MaxResourceNameLen)

// capWriter errors as soon as more than MaxResourceNameLen bytes are written.
type capWriter struct{ sb strings.Builder }

func (w *capWriter) Write(p []byte) (int, error) {
	if w.sb.Len()+len(p) > MaxResourceNameLen {
		return 0, errTooLong
	}
	return w.sb.Write(p)
}

const sampleSetup = "sample-setup"

type nameData struct {
	SetupName string
	Labels    map[string]string
}

var templateCache sync.Map // string -> *template.Template

func parseTemplate(tmpl string) (*template.Template, error) {
	if t, ok := templateCache.Load(tmpl); ok {
		return t.(*template.Template), nil
	}
	if len(tmpl) > MaxTemplateLen {
		return nil, fmt.Errorf("template: longer than %d bytes", MaxTemplateLen)
	}
	t, err := template.New("resource_name").Option("missingkey=zero").Parse(tmpl)
	if err != nil {
		return nil, fmt.Errorf("template: %w", err)
	}
	if err := checkTemplate(t); err != nil {
		return nil, fmt.Errorf("template: %w", err)
	}
	templateCache.Store(tmpl, t)
	return t, nil
}

// ResourceName derives the resource name of a setup (SET-4): a non-empty
// override wins; otherwise tmpl is executed with .SetupName and .Labels. A
// missing label renders as empty. An empty result is an error.
func ResourceName(tmpl, setupName string, labels map[string]string, override string) (string, error) {
	if override != "" {
		return override, nil
	}
	t, err := parseTemplate(tmpl)
	if err != nil {
		return "", err
	}
	if labels == nil {
		labels = map[string]string{}
	}
	var w capWriter
	if err := t.Execute(&w, nameData{SetupName: setupName, Labels: labels}); err != nil {
		return "", fmt.Errorf("template: %w", err)
	}
	name := w.sb.String()
	if name == "" {
		return "", errors.New("template produced an empty resource name")
	}
	return name, nil
}

// ValidateTemplate checks a template without rendering it: length, syntax and
// the allowed-node walk of checkTemplate. Settings.Load uses it so a bad
// stored value fails the poll with a clear error.
func ValidateTemplate(tmpl string) error {
	_, err := parseTemplate(tmpl)
	return err
}

// checkTemplate walks the parsed tree and allows only: text; field access on
// .SetupName and .Labels (.Labels.key, index .Labels "key"); the functions
// index, print and printf (printf formats without width, precision or *);
// if/else over those pipelines; and with over .Labels or a label. It rejects
// define, template, block, range, break, continue, variables and every other
// function, so no template can loop or recurse.
func checkTemplate(t *template.Template) error {
	if len(t.Templates()) > 1 {
		return errors.New("define, template and block are not allowed")
	}
	if t.Tree == nil || t.Tree.Root == nil {
		return nil
	}
	return checkNode(t.Tree.Root, false)
}

func checkNode(n parse.Node, inLabels bool) error {
	switch n := n.(type) {
	case nil:
		return nil
	case *parse.ListNode:
		if n == nil {
			return nil
		}
		for _, c := range n.Nodes {
			if err := checkNode(c, inLabels); err != nil {
				return err
			}
		}
		return nil
	case *parse.TextNode, *parse.CommentNode:
		return nil
	case *parse.ActionNode:
		return checkPipe(n.Pipe, inLabels)
	case *parse.IfNode:
		if err := checkPipe(n.Pipe, inLabels); err != nil {
			return err
		}
		if err := checkNode(n.List, inLabels); err != nil {
			return err
		}
		return checkNode(n.ElseList, inLabels)
	case *parse.WithNode:
		if err := checkPipe(n.Pipe, inLabels); err != nil {
			return err
		}
		if !isLabelsPipe(n.Pipe, inLabels) {
			return errors.New("with is only allowed over .Labels or a label")
		}
		if err := checkNode(n.List, true); err != nil {
			return err
		}
		return checkNode(n.ElseList, inLabels)
	case *parse.RangeNode:
		return errors.New("range is not allowed")
	case *parse.TemplateNode:
		return errors.New("template is not allowed")
	default:
		return fmt.Errorf("%s is not allowed", n.String())
	}
}

// isLabelsPipe reports whether a with pipeline is exactly .Labels (or, inside
// a with over .Labels, a single key of it).
func isLabelsPipe(p *parse.PipeNode, inLabels bool) bool {
	if len(p.Cmds) != 1 || len(p.Cmds[0].Args) != 1 {
		return false
	}
	f, ok := p.Cmds[0].Args[0].(*parse.FieldNode)
	if !ok {
		return false
	}
	if inLabels {
		return len(f.Ident) == 1
	}
	return f.Ident[0] == "Labels"
}

func checkPipe(p *parse.PipeNode, inLabels bool) error {
	if p == nil {
		return nil
	}
	if len(p.Decl) > 0 {
		return errors.New("variables are not allowed")
	}
	for _, c := range p.Cmds {
		if err := checkCommand(c, inLabels); err != nil {
			return err
		}
	}
	return nil
}

func checkCommand(c *parse.CommandNode, inLabels bool) error {
	if len(c.Args) == 0 {
		return errors.New("empty command")
	}
	switch first := c.Args[0].(type) {
	case *parse.IdentifierNode:
		switch first.Ident {
		case "index", "print":
		case "printf":
			if len(c.Args) < 2 {
				return errors.New("printf needs a format")
			}
			f, ok := c.Args[1].(*parse.StringNode)
			if !ok {
				return errors.New("printf format must be a constant string")
			}
			if err := checkFormat(f.Text); err != nil {
				return err
			}
		default:
			return fmt.Errorf("function %q is not allowed", first.Ident)
		}
	case *parse.FieldNode:
		if len(c.Args) > 1 {
			return errors.New("a field cannot take arguments")
		}
	default:
		if len(c.Args) > 1 {
			return errors.New("only index, print and printf take arguments")
		}
	}
	for _, a := range c.Args {
		if err := checkArg(a, inLabels); err != nil {
			return err
		}
	}
	return nil
}

func checkArg(a parse.Node, inLabels bool) error {
	switch a := a.(type) {
	case *parse.IdentifierNode:
		return nil // vetted by checkCommand when first; later ones are not functions
	case *parse.FieldNode:
		if inLabels {
			if len(a.Ident) != 1 {
				return errors.New("only a single label key may follow . inside with")
			}
			return nil
		}
		switch a.Ident[0] {
		case "SetupName":
			if len(a.Ident) != 1 {
				return errors.New(".SetupName has no fields")
			}
		case "Labels":
			if len(a.Ident) > 2 {
				return errors.New(".Labels takes one key")
			}
		default:
			return fmt.Errorf("field .%s is not available (use .SetupName or .Labels)", a.Ident[0])
		}
		return nil
	case *parse.DotNode:
		if !inLabels {
			return errors.New("a bare . is only allowed inside with .Labels")
		}
		return nil
	case *parse.StringNode, *parse.NumberNode, *parse.BoolNode:
		return nil
	case *parse.PipeNode:
		return checkPipe(a, inLabels)
	default:
		return fmt.Errorf("%s is not allowed", a.String())
	}
}

// checkFormat allows only plain verbs: every % is followed by one of v s d q
// or %, so no width, precision, flag or * can size the output.
func checkFormat(f string) error {
	for i := 0; i < len(f); i++ {
		if f[i] != '%' {
			continue
		}
		i++
		if i >= len(f) || !strings.ContainsRune("vsdq%", rune(f[i])) {
			return errors.New("printf formats may use only %v, %s, %d, %q and %%")
		}
	}
	return nil
}
