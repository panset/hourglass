// SPDX-License-Identifier: MIT
// Text handling for the Label selector box (UI-9, design B2). Selector terms come from the API
// (`label_index[].values[].selector`); this file only splits the box's text into whole terms and puts a
// term in, replaces one or takes one out. It does not build selector grammar.

/** Splits selector text on commas that are outside parentheses; terms are trimmed and empty ones dropped. */
export function splitTerms(text: string): string[] {
  const terms: string[] = [];
  let depth = 0;
  let start = 0;
  for (let i = 0; i <= text.length; i++) {
    const c = text[i];
    if (c === "(") depth++;
    else if (c === ")") depth = Math.max(0, depth - 1);
    else if (i === text.length || (c === "," && depth === 0)) {
      const term = text.slice(start, i).trim();
      if (term) terms.push(term);
      start = i + 1;
    }
  }
  return terms;
}

/** True for a plain `key=value` (or `key==value`) term of the given key; `key in (a,b)` and `key!=v` are not. */
function isEqualityTermOf(term: string, key: string): boolean {
  if (!term.startsWith(key)) return false;
  const rest = term.slice(key.length).trimStart();
  if (!rest.startsWith("=")) return false;
  const value = rest.replace(/^==?/, "").trimStart();
  return !/[\s,()=!]/.test(value);
}

/** The box text after a chip for `key` with selector `term` is activated: toggle off, replace, or append. */
export function applyChip(text: string, key: string, term: string): string {
  const terms = splitTerms(text);
  if (terms.includes(term)) return terms.filter((t) => t !== term).join(",");
  const at = terms.findIndex((t) => isEqualityTermOf(t, key));
  if (at >= 0) {
    return terms
      .map((t, i) => (i === at ? term : t))
      .filter((t, i) => i === at || !isEqualityTermOf(t, key))
      .join(",");
  }
  return [...terms, term].join(",");
}
