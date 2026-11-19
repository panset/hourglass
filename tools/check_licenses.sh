#!/usr/bin/env bash
# SPDX-License-Identifier: MIT
# Dependency license check (SPEC "License"). Usage: tools/check_licenses.sh [--write]
#   (no flag)  classify every Go module and npm package in use, fail on anything not
#              permissive, and fail if docs/third-party-licenses.md is out of date.
#   --write    regenerate docs/third-party-licenses.md (still fails on a bad license).
# Needs only bash, go and node. Go modules must be in the module cache (go mod download);
# npm packages are read from ui/node_modules (pnpm install --frozen-lockfile in ui/).
set -euo pipefail
cd "$(dirname "$0")/.."
command -v node >/dev/null || { echo "check_licenses: node is required" >&2; exit 2; }
exec node - "$@" <<'NODE'
// SPDX-License-Identifier: MIT
const fs = require('fs'), path = require('path'), cp = require('child_process');
const write = process.argv.includes('--write');
const root = process.cwd();
const OUT = 'docs/third-party-licenses.md';
const failures = [];
const fail = (m) => failures.push(m);

// ---- policy ----
const BASE = new Set(['MIT', 'BSD-2-Clause', 'BSD-3-Clause', 'Apache-2.0', 'ISC']);
const allow = []; // {id, name, why, used}
for (const line of fs.readFileSync('tools/licenses_allow.txt', 'utf8').split('\n')) {
  const t = line.trim();
  if (!t || t.startsWith('#')) continue;
  const [id, name, ...why] = t.split('|').map((s) => s.trim());
  if (!id || !name || !why.join('').trim()) { fail(`tools/licenses_allow.txt: malformed line: ${t}`); continue; }
  allow.push({ id, name, why: why.join('|'), used: false });
}
function idOk(id, name) {
  if (BASE.has(id)) return true;
  const e = allow.find((a) => a.id === id && a.name === name);
  if (e) { e.used = true; return true; }
  return false;
}
// SPDX expression: OR = any alternative allowed, AND = all allowed.
function exprOk(expr, name) {
  const s = expr.replace(/[()]/g, ' ').trim();
  return s.split(/\s+OR\s+/i).some((alt) => alt.split(/\s+AND\s+/i).every((id) => idOk(id.trim(), name)));
}

// ---- Go license text classification (fail closed) ----
function classifyText(raw) {
  const t = raw.replace(/\s+/g, ' ');
  const bad = [];
  if (/GNU AFFERO GENERAL PUBLIC LICENSE/i.test(t)) bad.push('AGPL');
  if (/GNU LESSER GENERAL PUBLIC LICENSE|GNU LIBRARY GENERAL PUBLIC LICENSE/i.test(t)) bad.push('LGPL');
  if (/GNU GENERAL PUBLIC LICENSE/i.test(t)) bad.push('GPL');
  if (/Mozilla Public License/i.test(t)) bad.push('MPL');
  if (/Server Side Public License/i.test(t)) bad.push('SSPL');
  if (/Business Source License/i.test(t)) bad.push('BUSL');
  if (/Eclipse Public License|Common Development and Distribution License|European Union Public Licen[sc]e/i.test(t)) bad.push('weak-copyleft');
  if (bad.length) return { copyleft: bad };
  const ids = [];
  if (/Apache License,? Version 2\.0/i.test(t)) ids.push('Apache-2.0');
  if (/Permission is hereby granted, free of charge, to any person obtaining a copy/i.test(t) && /THE SOFTWARE IS PROVIDED "AS IS"/i.test(t)) ids.push('MIT');
  if (/Permission to use, copy, modify, and\/or distribute this software for any purpose with or without fee/i.test(t)) {
    ids.push(/THE SOFTWARE IS PROVIDED "AS IS" AND THE AUTHOR DISCLAIMS/i.test(t) && !/with or without fee is hereby granted, provided/i.test(t) ? '0BSD' : 'ISC');
  }
  if (/Redistribution and use in source and binary forms, with or without modification, are permitted provided that/i.test(t)) {
    ids.push(/Neither the name|endorse or promote/i.test(t) ? 'BSD-3-Clause' : 'BSD-2-Clause');
  }
  if (/This is free and unencumbered software released into the public domain/i.test(t)) ids.push('Unlicense');
  if (/CC0 1\.0 Universal/i.test(t)) ids.push('CC0-1.0');
  if (/Blue Oak Model License/i.test(t)) ids.push('BlueOak-1.0.0');
  return { ids };
}
const LICFILE = /^(licen[sc]e|copying|unlicense)([-._].*)?$/i;
const PRIMARY = /^(licen[sc]e|copying|unlicense)(\.(txt|md|markdown))?$/i;

// ---- Go ----
const goMods = new Map();
function goInventory() {
  for (const goos of ['linux', 'darwin']) {
    let out;
    try {
      out = cp.execFileSync('go', ['list', '-deps', '-test', '-f', '{{with .Module}}{{if not .Main}}{{.Path}}\t{{.Version}}\t{{.Dir}}{{end}}{{end}}', './...'],
        { env: { ...process.env, GOOS: goos, GOARCH: 'amd64', CGO_ENABLED: '0' }, encoding: 'utf8', stdio: ['ignore', 'pipe', 'pipe'], maxBuffer: 1 << 26 });
    } catch (e) { fail(`go list failed (${goos}); run "go mod download": ${(e.stderr || e.message).toString().trim().split('\n').slice(-3).join(' / ')}`); return; }
    for (const l of out.split('\n')) {
      if (!l) continue;
      const [p, v, d] = l.split('\t');
      if (!goMods.has(p)) goMods.set(p, { path: p, version: v, dir: d });
    }
  }
}
function goClassify(m) {
  if (!m.dir || !fs.existsSync(m.dir)) { fail(`go ${m.path}@${m.version}: not in module cache; run "go mod download"`); return 'MISSING'; }
  const files = fs.readdirSync(m.dir).filter((f) => LICFILE.test(f) && fs.statSync(path.join(m.dir, f)).isFile());
  const primary = files.filter((f) => PRIMARY.test(f));
  if (!primary.length) { fail(`go ${m.path}@${m.version}: no LICENSE/COPYING file found`); return 'MISSING'; }
  const found = new Set();
  let ok = true;
  for (const f of files) {
    // Third-party inventories (e.g. modernc LICENSE-3RD-PARTY.md) list licenses of modules that
    // are not built here; the modules that are built are checked on their own.
    if (/3RD-PARTY|THIRD[-_]PARTY/i.test(f)) continue;
    const r = classifyText(fs.readFileSync(path.join(m.dir, f), 'utf8'));
    if (r.copyleft) { fail(`go ${m.path}@${m.version}: ${f} contains ${r.copyleft.join('/')} license text`); ok = false; continue; }
    if (!PRIMARY.test(f)) continue; // secondary files (third-party notes, patents) only checked for copyleft
    if (!r.ids.length) { fail(`go ${m.path}@${m.version}: ${f} is not a recognised license text (unknown, fails closed)`); ok = false; continue; }
    r.ids.forEach((i) => found.add(i));
  }
  if (!ok) return 'DISALLOWED';
  const ids = [...found].sort();
  for (const i of ids) if (!idOk(i, m.path)) { fail(`go ${m.path}@${m.version}: license ${i} is not permitted`); ok = false; }
  return ids.join(' AND ');
}

// ---- npm (pnpm-lock.yaml v9) ----
function npmInventory(dir) {
  const lock = path.join(dir, 'pnpm-lock.yaml');
  if (!fs.existsSync(lock)) { if (fs.existsSync(path.join(dir, 'package.json'))) fail(`${dir}: package.json without pnpm-lock.yaml; cannot enumerate packages`); return []; }
  const text = fs.readFileSync(lock, 'utf8');
  const sec = (text.split(/^packages:\n/m)[1] || '').split(/^snapshots:/m)[0];
  const entries = [...sec.matchAll(/^  '?([^\n]+?)'?:\n((?:    .*\n)*)/gm)].map((m) => {
    const k = m[1], i = k.lastIndexOf('@');
    return { name: k.slice(0, i), version: k.slice(i + 1), platform: /^\s+(os|cpu|libc):/m.test(m[2]) };
  });
  if (!entries.length) { fail(`${lock}: no packages parsed`); return []; }
  const store = path.join(dir, 'node_modules', '.pnpm');
  const stored = fs.existsSync(store) ? fs.readdirSync(store) : [];
  if (!stored.length) fail(`${dir}: node_modules not installed; run "pnpm install --frozen-lockfile" in ${dir} (the lockfile records no license metadata)`);
  const read = (e) => {
    const pre = `${e.name.replace('/', '+')}@${e.version}`;
    const d = stored.find((s) => s === pre || s.startsWith(pre + '_') || s.startsWith(pre + '('));
    if (!d) return null;
    const pj = path.join(store, d, 'node_modules', e.name, 'package.json');
    if (!fs.existsSync(pj)) return null;
    const j = JSON.parse(fs.readFileSync(pj, 'utf8'));
    let l = j.license ?? j.licenses;
    if (Array.isArray(l)) l = l.map((x) => x.type || x).join(' OR ');
    else if (l && typeof l === 'object') l = l.type;
    return typeof l === 'string' && l.trim() ? l.trim() : 'UNKNOWN';
  };
  const lic = new Map();
  for (const e of entries) { const l = read(e); if (l) lic.set(e, l); }
  for (const e of entries) {
    if (lic.has(e)) { e.license = lic.get(e); continue; }
    if (!stored.length) { e.license = 'MISSING'; continue; }
    // Platform-specific optional package not installed on this OS: take the license of an
    // installed sibling in the same scope (e.g. @esbuild/*), else fail closed.
    const scope = e.name.startsWith('@') ? e.name.split('/')[0] + '/' : null;
    const sib = e.platform && scope ? [...lic].find(([s]) => s.name.startsWith(scope) && s.platform) || [...lic].find(([s]) => s.name.startsWith(scope)) : null;
    if (sib) { e.license = sib[1]; e.inferred = true; }
    else { e.license = 'MISSING'; fail(`npm ${e.name}@${e.version}: not installed and no sibling to infer from; run "pnpm install --frozen-lockfile" in ${dir}`); }
  }
  return entries;
}
function npmCheck(e, where) {
  if (e.license === 'MISSING') return;
  if (e.license === 'UNKNOWN') { fail(`npm ${e.name}@${e.version} (${where}): no license field`); return; }
  if (!exprOk(e.license, e.name)) fail(`npm ${e.name}@${e.version} (${where}): license "${e.license}" is not permitted`);
}

// ---- run ----
goInventory();
const goRows = [...goMods.values()].sort((a, b) => a.path.localeCompare(b.path)).map((m) => ({ ...m, license: goClassify(m) }));
const npmSets = [];
for (const dir of ['ui', 'e2e/browser']) if (fs.existsSync(dir)) npmSets.push([dir, npmInventory(dir).sort((a, b) => (a.name + a.version).localeCompare(b.name + b.version))]);
for (const [dir, rows] of npmSets) rows.forEach((e) => npmCheck(e, dir));
for (const a of allow) if (!a.used) fail(`tools/licenses_allow.txt: unused entry "${a.id}|${a.name}" (remove it; exceptions are listed only if they occur)`);

// ---- inventory doc ----
const esc = (s) => s.replace(/\|/g, '\\|');
let doc = '# Third-party licenses\n\nGenerated by `tools/check_licenses.sh --write`; do not edit by hand. CI fails if this file is out of date.\nPolicy: MIT, BSD-2-Clause, BSD-3-Clause, Apache-2.0, ISC, plus the justified exceptions in `tools/licenses_allow.txt`.\n\n';
doc += `## Go modules (build and test dependencies of the main module)\n\n| Module | Version | License |\n| --- | --- | --- |\n`;
for (const m of goRows) doc += `| ${m.path} | ${m.version} | ${esc(m.license)} |\n`;
for (const [dir, rows] of npmSets) {
  doc += `\n## npm packages (${dir}/pnpm-lock.yaml, production and dev)\n\n| Package | Version | License |\n| --- | --- | --- |\n`;
  for (const e of rows) doc += `| ${e.name} | ${e.version} | ${esc(e.license)} |\n`;
}
if (write) { fs.writeFileSync(OUT, doc); console.log(`wrote ${OUT}`); }
else if (!fs.existsSync(OUT) || fs.readFileSync(OUT, 'utf8') !== doc) fail(`${OUT} is out of date; run tools/check_licenses.sh --write`);

if (failures.length) { console.error('license check FAILED:'); failures.forEach((f) => console.error('  - ' + f)); process.exit(1); }
console.log(`license check ok: ${goRows.length} Go modules, ${npmSets.reduce((n, [, r]) => n + r.length, 0)} npm packages`);
NODE
