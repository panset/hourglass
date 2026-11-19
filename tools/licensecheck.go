// SPDX-License-Identifier: MIT

//go:build ignore

// licensecheck is the dependency license check (SPEC "License"); run it
// through tools/check_licenses.sh. It needs only the go tool.
//
//	check (default): classifies every Go module in use (from the module
//	  cache), verifies that each npm lockfile hash equals the one recorded in
//	  tools/npm_licenses.json, evaluates the recorded npm licenses against the
//	  policy and compares docs/third-party-licenses.md with what it would write.
//	--write: also reads the npm packages (ui: pnpm-lock.yaml plus
//	  ui/node_modules; e2e/browser: package-lock.json, with node_modules as a
//	  fallback), rewrites tools/npm_licenses.json and docs/third-party-licenses.md.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const (
	outDoc   = "docs/third-party-licenses.md"
	dataFile = "tools/npm_licenses.json"
)

// npmProjects: directory and the lockfile that pins it, in output order.
var npmProjects = []struct{ dir, lock string }{
	{"ui", "pnpm-lock.yaml"},
	{"e2e/browser", "package-lock.json"},
}

var failures []string

func fail(format string, a ...any) { failures = append(failures, fmt.Sprintf(format, a...)) }

// ---- policy ----

var base = map[string]bool{"MIT": true, "BSD-2-Clause": true, "BSD-3-Clause": true, "Apache-2.0": true, "ISC": true}

type allowEntry struct {
	id, name string
	used     bool
}

var allow []*allowEntry

func loadAllow() {
	b, err := os.ReadFile("tools/licenses_allow.txt")
	if err != nil {
		fail("tools/licenses_allow.txt: %v", err)
		return
	}
	for _, line := range strings.Split(string(b), "\n") {
		t := strings.TrimSpace(line)
		if t == "" || strings.HasPrefix(t, "#") {
			continue
		}
		parts := strings.Split(t, "|")
		if len(parts) < 3 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" || strings.TrimSpace(strings.Join(parts[2:], "")) == "" {
			fail("tools/licenses_allow.txt: malformed line: %s", t)
			continue
		}
		allow = append(allow, &allowEntry{id: strings.TrimSpace(parts[0]), name: strings.TrimSpace(parts[1])})
	}
}

func idOK(id, name string) bool {
	if base[id] {
		return true
	}
	for _, a := range allow {
		if a.id == id && a.name == name {
			a.used = true
			return true
		}
	}
	return false
}

var (
	reOr  = regexp.MustCompile(`(?i)\s+OR\s+`)
	reAnd = regexp.MustCompile(`(?i)\s+AND\s+`)
)

// exprOK evaluates an SPDX expression: OR needs any alternative allowed, AND all of them.
func exprOK(expr, name string) bool {
	s := strings.TrimSpace(strings.NewReplacer("(", " ", ")", " ").Replace(expr))
	for _, alt := range reOr.Split(s, -1) {
		all := true
		for _, id := range reAnd.Split(alt, -1) {
			if !idOK(strings.TrimSpace(id), name) {
				all = false
			}
		}
		if all {
			return true
		}
	}
	return false
}

// ---- Go license text classification (fails closed) ----

func rx(p string) *regexp.Regexp { return regexp.MustCompile("(?i)" + p) }

var (
	reSpace     = regexp.MustCompile(`\s+`)
	reAGPL      = rx(`GNU AFFERO GENERAL PUBLIC LICENSE`)
	reLGPL      = rx(`GNU LESSER GENERAL PUBLIC LICENSE|GNU LIBRARY GENERAL PUBLIC LICENSE`)
	reGPL       = rx(`GNU GENERAL PUBLIC LICENSE`)
	reMPL       = rx(`Mozilla Public License`)
	reSSPL      = rx(`Server Side Public License`)
	reBUSL      = rx(`Business Source License`)
	reWeak      = rx(`Eclipse Public License|Common Development and Distribution License|European Union Public Licen[sc]e`)
	reApache    = rx(`Apache License,? Version 2\.0`)
	reMITPerm   = rx(`Permission is hereby granted, free of charge, to any person obtaining a copy`)
	reAsIs      = rx(`THE SOFTWARE IS PROVIDED "AS IS"`)
	reISCPerm   = rx(`Permission to use, copy, modify, and/or distribute this software for any purpose with or without fee`)
	reBSD0      = rx(`THE SOFTWARE IS PROVIDED "AS IS" AND THE AUTHOR DISCLAIMS`)
	reISCGrant  = rx(`with or without fee is hereby granted, provided`)
	reBSD       = rx(`Redistribution and use in source and binary forms, with or without modification, are permitted provided that`)
	reBSD3      = rx(`Neither the name|endorse or promote`)
	reUnlicense = rx(`This is free and unencumbered software released into the public domain`)
	reCC0       = rx(`CC0 1\.0 Universal`)
	reBlueOak   = rx(`Blue Oak Model License`)
	reLicFile   = rx(`^(licen[sc]e|copying|unlicense)([-._].*)?$`)
	rePrimary   = rx(`^(licen[sc]e|copying|unlicense)(\.(txt|md|markdown))?$`)
	reThirdFile = rx(`3RD-PARTY|THIRD[-_]PARTY`)
)

func classifyText(raw string) (ids, copyleft []string) {
	t := reSpace.ReplaceAllString(raw, " ")
	for _, c := range []struct {
		re   *regexp.Regexp
		name string
	}{{reAGPL, "AGPL"}, {reLGPL, "LGPL"}, {reGPL, "GPL"}, {reMPL, "MPL"}, {reSSPL, "SSPL"}, {reBUSL, "BUSL"}, {reWeak, "weak-copyleft"}} {
		if c.re.MatchString(t) {
			copyleft = append(copyleft, c.name)
		}
	}
	if len(copyleft) > 0 {
		return nil, copyleft
	}
	if reApache.MatchString(t) {
		ids = append(ids, "Apache-2.0")
	}
	if reMITPerm.MatchString(t) && reAsIs.MatchString(t) {
		ids = append(ids, "MIT")
	}
	if reISCPerm.MatchString(t) {
		if reBSD0.MatchString(t) && !reISCGrant.MatchString(t) {
			ids = append(ids, "0BSD")
		} else {
			ids = append(ids, "ISC")
		}
	}
	if reBSD.MatchString(t) {
		if reBSD3.MatchString(t) {
			ids = append(ids, "BSD-3-Clause")
		} else {
			ids = append(ids, "BSD-2-Clause")
		}
	}
	if reUnlicense.MatchString(t) {
		ids = append(ids, "Unlicense")
	}
	if reCC0.MatchString(t) {
		ids = append(ids, "CC0-1.0")
	}
	if reBlueOak.MatchString(t) {
		ids = append(ids, "BlueOak-1.0.0")
	}
	return ids, nil
}

// ---- Go modules ----

type goMod struct{ path, version, dir, license string }

func goInventory() []*goMod {
	mods := map[string]*goMod{}
	for _, goos := range []string{"linux", "darwin"} {
		cmd := exec.Command("go", "list", "-deps", "-test", "-f",
			"{{with .Module}}{{if not .Main}}{{.Path}}\t{{.Version}}\t{{.Dir}}{{end}}{{end}}", "./...")
		cmd.Env = append(os.Environ(), "GOOS="+goos, "GOARCH=amd64", "CGO_ENABLED=0")
		var stderr strings.Builder
		cmd.Stderr = &stderr
		out, err := cmd.Output()
		if err != nil {
			tail := strings.Split(strings.TrimSpace(stderr.String()), "\n")
			if len(tail) > 3 {
				tail = tail[len(tail)-3:]
			}
			fail("go list failed (%s); run \"go mod download\": %s", goos, strings.Join(tail, " / "))
			return nil
		}
		for _, l := range strings.Split(string(out), "\n") {
			f := strings.Split(l, "\t")
			if len(f) != 3 {
				continue
			}
			if _, ok := mods[f[0]]; !ok {
				mods[f[0]] = &goMod{path: f[0], version: f[1], dir: f[2]}
			}
		}
	}
	var rows []*goMod
	for _, m := range mods {
		rows = append(rows, m)
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].path < rows[j].path })
	for _, m := range rows {
		m.license = goClassify(m)
	}
	return rows
}

func goClassify(m *goMod) string {
	st, err := os.Stat(m.dir)
	if m.dir == "" || err != nil || !st.IsDir() {
		fail("go %s@%s: not in module cache; run \"go mod download\"", m.path, m.version)
		return "MISSING"
	}
	entries, _ := os.ReadDir(m.dir)
	var files []string
	primary := 0
	for _, e := range entries {
		if reLicFile.MatchString(e.Name()) && !e.IsDir() {
			files = append(files, e.Name())
			if rePrimary.MatchString(e.Name()) {
				primary++
			}
		}
	}
	if primary == 0 {
		fail("go %s@%s: no LICENSE/COPYING file found", m.path, m.version)
		return "MISSING"
	}
	found := map[string]bool{}
	ok := true
	for _, f := range files {
		// Third-party inventories (for example modernc LICENSE-3RD-PARTY.md) list licenses of
		// modules that are not built here; the modules that are built are checked on their own.
		if reThirdFile.MatchString(f) {
			continue
		}
		b, err := os.ReadFile(filepath.Join(m.dir, f))
		if err != nil {
			fail("go %s@%s: %v", m.path, m.version, err)
			ok = false
			continue
		}
		ids, cl := classifyText(string(b))
		if len(cl) > 0 {
			fail("go %s@%s: %s contains %s license text", m.path, m.version, f, strings.Join(cl, "/"))
			ok = false
			continue
		}
		if !rePrimary.MatchString(f) {
			continue // secondary files only checked for copyleft
		}
		if len(ids) == 0 {
			fail("go %s@%s: %s is not a recognised license text (unknown, fails closed)", m.path, m.version, f)
			ok = false
			continue
		}
		for _, i := range ids {
			found[i] = true
		}
	}
	if !ok {
		return "DISALLOWED"
	}
	var ids []string
	for i := range found {
		ids = append(ids, i)
	}
	sort.Strings(ids)
	for _, i := range ids {
		if !idOK(i, m.path) {
			fail("go %s@%s: license %s is not permitted", m.path, m.version, i)
			ok = false
		}
	}
	return strings.Join(ids, " AND ")
}

// ---- npm: the recorded inventory ----

type npmPkg struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	License string `json:"license"`
}

type npmProject struct {
	Lockfile       string   `json:"lockfile"`
	LockfileSHA256 string   `json:"lockfile_sha256"`
	Packages       []npmPkg `json:"packages"`
}

type npmData struct {
	Comment  string                `json:"comment"`
	Projects map[string]npmProject `json:"projects"`
}

const dataComment = "Generated by tools/check_licenses.sh --write; do not edit by hand. The plain check verifies the lockfile hashes and evaluates these licenses against the policy."

func sha256File(p string) (string, error) {
	b, err := os.ReadFile(p)
	if err != nil {
		return "", err
	}
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:]), nil
}

func exists(p string) bool { _, err := os.Stat(p); return err == nil }

func sortPkgs(p []npmPkg) {
	sort.Slice(p, func(i, j int) bool { return p[i].Name+"\x00"+p[i].Version < p[j].Name+"\x00"+p[j].Version })
}

func readData() npmData {
	var d npmData
	b, err := os.ReadFile(dataFile)
	if err != nil {
		fail("%s: %v; run tools/check_licenses.sh --write", dataFile, err)
		return d
	}
	if err := json.Unmarshal(b, &d); err != nil {
		fail("%s: %v", dataFile, err)
	}
	return d
}

func verifyData(d npmData) {
	for _, pr := range npmProjects {
		lock := filepath.Join(pr.dir, pr.lock)
		rec, recorded := d.Projects[pr.dir]
		switch {
		case !exists(pr.dir):
			if recorded {
				fail("%s: recorded in %s but the directory is gone; run tools/check_licenses.sh --write", pr.dir, dataFile)
			}
		case !exists(lock):
			if exists(filepath.Join(pr.dir, "package.json")) {
				fail("%s: package.json without %s; cannot enumerate packages", pr.dir, pr.lock)
			}
		case !recorded:
			fail("%s: not recorded in %s; run tools/check_licenses.sh --write", pr.dir, dataFile)
		default:
			h, err := sha256File(lock)
			if err != nil {
				fail("%s: %v", lock, err)
			} else if h != rec.LockfileSHA256 {
				fail("%s changed since %s was written (recorded sha256 %s, now %s); run tools/check_licenses.sh --write", lock, dataFile, rec.LockfileSHA256, h)
			}
		}
	}
	for dir := range d.Projects {
		known := false
		for _, pr := range npmProjects {
			known = known || pr.dir == dir
		}
		if !known {
			fail("%s: unknown project %q", dataFile, dir)
		}
	}
	for dir, rec := range d.Projects {
		for _, e := range rec.Packages {
			switch e.License {
			case "", "MISSING", "UNKNOWN":
				fail("npm %s@%s (%s): no license recorded", e.Name, e.Version, dir)
			default:
				if !exprOK(e.License, e.Name) {
					fail("npm %s@%s (%s): license %q is not permitted", e.Name, e.Version, dir, e.License)
				}
			}
		}
	}
}

// ---- npm: reading packages (--write) ----

type pkgJSON struct {
	License  json.RawMessage `json:"license"`
	Licenses json.RawMessage `json:"licenses"`
}

func licenseOf(raw json.RawMessage) string {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return strings.TrimSpace(s)
	}
	var o struct{ Type string }
	if json.Unmarshal(raw, &o) == nil && o.Type != "" {
		return o.Type
	}
	var arr []json.RawMessage
	if json.Unmarshal(raw, &arr) == nil {
		var parts []string
		for _, a := range arr {
			if l := licenseOf(a); l != "" {
				parts = append(parts, l)
			}
		}
		return strings.Join(parts, " OR ")
	}
	return ""
}

func readPkgJSON(p string) (string, bool) {
	b, err := os.ReadFile(p)
	if err != nil {
		return "", false
	}
	var j pkgJSON
	if json.Unmarshal(b, &j) != nil {
		return "UNKNOWN", true
	}
	l := ""
	if len(j.License) > 0 {
		l = licenseOf(j.License)
	}
	if l == "" && len(j.Licenses) > 0 {
		l = licenseOf(j.Licenses)
	}
	if l == "" {
		l = "UNKNOWN"
	}
	return l, true
}

var rePnpmEntry = regexp.MustCompile(`(?m)^  '?([^\n]+?)'?:\n((?:    .*\n)*)`)
var rePnpmPlatform = regexp.MustCompile(`(?m)^\s+(os|cpu|libc):`)

func pnpmPackages(dir string) []npmPkg {
	lock := filepath.Join(dir, "pnpm-lock.yaml")
	b, err := os.ReadFile(lock)
	if err != nil {
		fail("%s: %v", lock, err)
		return nil
	}
	text := string(b)
	sec := ""
	if i := strings.Index(text, "\npackages:\n"); i >= 0 {
		sec = text[i+len("\npackages:\n"):]
	} else if strings.HasPrefix(text, "packages:\n") {
		sec = text[len("packages:\n"):]
	}
	if i := strings.Index(sec, "\nsnapshots:"); i >= 0 {
		sec = sec[:i+1]
	}
	type entry struct {
		name, version string
		platform      bool
		license       string
		have          bool
	}
	var entries []*entry
	for _, m := range rePnpmEntry.FindAllStringSubmatch(sec, -1) {
		k := m[1]
		i := strings.LastIndex(k, "@")
		entries = append(entries, &entry{name: k[:i], version: k[i+1:], platform: rePnpmPlatform.MatchString(m[2])})
	}
	if len(entries) == 0 {
		fail("%s: no packages parsed", lock)
		return nil
	}
	store := filepath.Join(dir, "node_modules", ".pnpm")
	stored, _ := os.ReadDir(store)
	if len(stored) == 0 {
		fail("%s: node_modules not installed; run \"pnpm install --frozen-lockfile\" in %s (the lockfile records no license metadata)", dir, dir)
		return nil
	}
	for _, e := range entries {
		pre := strings.ReplaceAll(e.name, "/", "+") + "@" + e.version
		for _, s := range stored {
			n := s.Name()
			if n == pre || strings.HasPrefix(n, pre+"_") || strings.HasPrefix(n, pre+"(") {
				if l, ok := readPkgJSON(filepath.Join(store, n, "node_modules", filepath.FromSlash(e.name), "package.json")); ok {
					e.license, e.have = l, true
					break
				}
			}
		}
	}
	var out []npmPkg
	for _, e := range entries {
		if !e.have {
			// Platform-specific optional package not installed on this OS: take the license of
			// an installed sibling in the same scope (for example @esbuild/*), else fail closed.
			if strings.HasPrefix(e.name, "@") && e.platform {
				scope := e.name[:strings.Index(e.name, "/")+1]
				for _, s := range entries {
					if s.have && s.platform && strings.HasPrefix(s.name, scope) {
						e.license, e.have = s.license, true
						break
					}
				}
				if !e.have {
					for _, s := range entries {
						if s.have && strings.HasPrefix(s.name, scope) {
							e.license, e.have = s.license, true
							break
						}
					}
				}
			}
			if !e.have {
				fail("npm %s@%s: not installed and no sibling to infer from; run \"pnpm install --frozen-lockfile\" in %s", e.name, e.version, dir)
				e.license = "MISSING"
			}
		}
		out = append(out, npmPkg{Name: e.name, Version: e.version, License: e.license})
	}
	return out
}

func npmLockPackages(dir string) []npmPkg {
	lock := filepath.Join(dir, "package-lock.json")
	b, err := os.ReadFile(lock)
	if err != nil {
		fail("%s: %v", lock, err)
		return nil
	}
	var j struct {
		Packages map[string]struct {
			Name    string          `json:"name"`
			Version string          `json:"version"`
			License json.RawMessage `json:"license"`
			Link    bool            `json:"link"`
		} `json:"packages"`
	}
	if err := json.Unmarshal(b, &j); err != nil {
		fail("%s: %v", lock, err)
		return nil
	}
	var out []npmPkg
	for k, p := range j.Packages {
		if k == "" || p.Link {
			continue
		}
		name := p.Name
		if i := strings.LastIndex(k, "node_modules/"); i >= 0 {
			name = k[i+len("node_modules/"):]
		}
		lic := ""
		if len(p.License) > 0 {
			lic = licenseOf(p.License)
		}
		if lic == "" {
			if l, ok := readPkgJSON(filepath.Join(dir, filepath.FromSlash(k), "package.json")); ok {
				lic = l
			}
		}
		if lic == "" {
			fail("npm %s@%s (%s): no license in the lockfile and not installed; run \"npm ci\" in %s", name, p.Version, dir, dir)
			lic = "MISSING"
		}
		out = append(out, npmPkg{Name: name, Version: p.Version, License: lic})
	}
	return out
}

func writeData() npmData {
	d := npmData{Comment: dataComment, Projects: map[string]npmProject{}}
	for _, pr := range npmProjects {
		lock := filepath.Join(pr.dir, pr.lock)
		if !exists(pr.dir) {
			continue
		}
		if !exists(lock) {
			if exists(filepath.Join(pr.dir, "package.json")) {
				fail("%s: package.json without %s; cannot enumerate packages", pr.dir, pr.lock)
			}
			continue
		}
		h, err := sha256File(lock)
		if err != nil {
			fail("%s: %v", lock, err)
			continue
		}
		var pk []npmPkg
		if pr.lock == "pnpm-lock.yaml" {
			pk = pnpmPackages(pr.dir)
		} else {
			pk = npmLockPackages(pr.dir)
		}
		sortPkgs(pk)
		d.Projects[pr.dir] = npmProject{Lockfile: pr.lock, LockfileSHA256: h, Packages: pk}
	}
	return d
}

// ---- inventory document ----

func esc(s string) string { return strings.ReplaceAll(s, "|", `\|`) }

func renderDoc(goRows []*goMod, d npmData) string {
	var sb strings.Builder
	sb.WriteString("# Third-party licenses\n\nGenerated by `tools/check_licenses.sh --write`; do not edit by hand. CI fails if this file is out of date.\n" +
		"Policy: MIT, BSD-2-Clause, BSD-3-Clause, Apache-2.0, ISC, plus the justified exceptions in `tools/licenses_allow.txt`.\n\n")
	sb.WriteString("## Go modules (build and test dependencies of the main module)\n\n| Module | Version | License |\n| --- | --- | --- |\n")
	for _, m := range goRows {
		fmt.Fprintf(&sb, "| %s | %s | %s |\n", m.path, m.version, esc(m.license))
	}
	for _, pr := range npmProjects {
		rec, ok := d.Projects[pr.dir]
		if !ok {
			continue
		}
		fmt.Fprintf(&sb, "\n## npm packages (%s/%s, production and dev)\n\n| Package | Version | License |\n| --- | --- | --- |\n", pr.dir, pr.lock)
		pk := append([]npmPkg(nil), rec.Packages...)
		sortPkgs(pk)
		for _, e := range pk {
			fmt.Fprintf(&sb, "| %s | %s | %s |\n", e.Name, e.Version, esc(e.License))
		}
	}
	return sb.String()
}

func main() {
	write := len(os.Args) > 1 && os.Args[1] == "--write"
	if len(os.Args) > 1 && !write {
		fmt.Fprintln(os.Stderr, "usage: check_licenses.sh [--write]")
		os.Exit(2)
	}
	loadAllow()
	goRows := goInventory()
	var d npmData
	if write {
		d = writeData()
	} else {
		d = readData()
		verifyData(d)
	}
	if write {
		// Evaluate the freshly read licenses against the policy before recording them.
		verifyData(d)
	}
	for _, a := range allow {
		if !a.used {
			fail("tools/licenses_allow.txt: unused entry \"%s|%s\" (remove it; exceptions are listed only if they occur)", a.id, a.name)
		}
	}
	doc := renderDoc(goRows, d)
	if write {
		if len(failures) == 0 {
			b, _ := json.MarshalIndent(d, "", "  ")
			if err := os.WriteFile(dataFile, append(b, '\n'), 0o644); err != nil {
				fail("%v", err)
			}
			if err := os.WriteFile(outDoc, []byte(doc), 0o644); err != nil {
				fail("%v", err)
			}
			fmt.Printf("wrote %s and %s\n", dataFile, outDoc)
		}
	} else if cur, err := os.ReadFile(outDoc); err != nil || string(cur) != doc {
		fail("%s is out of date; run tools/check_licenses.sh --write", outDoc)
	}
	if len(failures) > 0 {
		fmt.Fprintln(os.Stderr, "license check FAILED:")
		for _, f := range failures {
			fmt.Fprintln(os.Stderr, "  - "+f)
		}
		os.Exit(1)
	}
	n := 0
	for _, p := range d.Projects {
		n += len(p.Packages)
	}
	fmt.Printf("license check ok: %d Go modules, %d npm packages\n", len(goRows), n)
}
