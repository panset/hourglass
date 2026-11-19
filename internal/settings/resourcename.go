// SPDX-License-Identifier: MIT

package settings

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"text/template"
)

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
	t, err := template.New("resource_name").Option("missingkey=zero").Parse(tmpl)
	if err != nil {
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
	var sb strings.Builder
	if err := t.Execute(&sb, nameData{SetupName: setupName, Labels: labels}); err != nil {
		return "", fmt.Errorf("template: %w", err)
	}
	name := sb.String()
	if name == "" {
		return "", errors.New("template produced an empty resource name")
	}
	return name, nil
}
