// SPDX-License-Identifier: MIT
// Suggestions for the Label selector box (UI-21, UI-22). Pure functions: which token the caret is in, what to
// suggest there from `label_index`, and what a pick does to the text. Nothing here parses or validates a
// selector: the API is the only judge. A value is inserted only as the API wrote it; no escaping is built.
import type { SetupList } from "../usage/holdApi";

export type LabelIndex = NonNullable<SetupList["label_index"]>;
type IndexKey = LabelIndex[number];

/** At most this many options are rendered; the rest are counted in `Suggestions.more`. */
export const MAX_OPTIONS = 50;
/** A placeholder example longer than this would be cut by the box. */
const PLACEHOLDER_TERM_MAX = 16;
/** A key option shows this many of its values as a sample. */
const SAMPLE_VALUES = 3;

/**
 * The placeholder of the selector box: "Type or pick a label, e.g. size=large" with the first term of the
 * index that is short enough to read in full, the words alone when no term is, and none for an empty index.
 */
export function placeholderFrom(index: LabelIndex, narrow = false): string | undefined {
  if (index.length === 0) return undefined;
  // In a narrow window the box cannot hold the example: the words alone read whole.
  if (narrow) return "Type or pick a label";
  for (const k of index) {
    for (const v of k.values) {
      if (v.selector && v.value !== "" && v.selector.length <= PLACEHOLDER_TERM_MAX) return `Type or pick a label, e.g. ${v.selector}`;
    }
  }
  return "Type or pick a label";
}

export type Position = "key" | "notKey" | "operator" | "value" | "list" | "none";

/** Where the caret is inside the selector text. Offsets are indexes into the whole text. */
export interface Context {
  position: Position;
  /** The key being written about, for `operator`, `value` and `list`; "" otherwise. */
  key: string;
  /** The operator before a `value` position. */
  operator: "=" | "==" | "!=" | "";
  /** The text of the token from its start to the caret: what suggestions are matched against. */
  partial: string;
  /** The whole token under the caret, also the part after the caret. A pick replaces it. */
  start: number;
  end: number;
  /** The requirement the caret is in: between top-level commas. */
  reqStart: number;
  reqEnd: number;
  /** `operator`: where the key ends. */
  keyEnd: number;
  /** `list`: the items already in the list, without the token under the caret. */
  listed: string[];
  /** `key`: nothing follows the token in its requirement, so a pick may add to it. */
  bare: boolean;
}

const isSpace = (c: string) => c === " " || c === "\t" || c === "\n" || c === "\r";
const isSymbol = (c: string) => c === "," || c === "(" || c === ")" || c === "=" || c === "!";
const isWord = (c: string | undefined): c is string => c !== undefined && !isSpace(c) && !isSymbol(c);

interface Token {
  text: string;
  word: boolean;
  end: number;
}

/** Words (runs of token characters) and single symbols of `text[from, to)`. */
function tokensOf(text: string, from: number, to: number): Token[] {
  const out: Token[] = [];
  let i = from;
  while (i < to) {
    const c = text[i]!;
    if (isSpace(c)) {
      i++;
    } else if (isSymbol(c)) {
      i++;
      out.push({ text: c, word: false, end: i });
    } else {
      let j = i;
      while (j < to && isWord(text[j])) j++;
      out.push({ text: text.slice(i, j), word: true, end: j });
      i = j;
    }
  }
  return out;
}

/** Classifies the caret position (see DESIGN-R3 "What is suggested at each caret position"). */
export function contextAt(text: string, caretAt: number): Context {
  const caret = Math.max(0, Math.min(caretAt, text.length));
  // The requirement: a comma inside parentheses does not split.
  let depth = 0;
  let reqStart = 0;
  for (let i = 0; i < caret; i++) {
    const c = text[i];
    if (c === "(") depth++;
    else if (c === ")") depth = Math.max(0, depth - 1);
    else if (c === "," && depth === 0) reqStart = i + 1;
  }
  let reqEnd = text.length;
  for (let i = caret, d = depth; i < text.length; i++) {
    const c = text[i];
    if (c === "(") d++;
    else if (c === ")") d = Math.max(0, d - 1);
    else if (c === "," && d === 0) {
      reqEnd = i;
      break;
    }
  }
  let start = caret;
  while (start > reqStart && isWord(text[start - 1])) start--;
  let end = caret;
  while (end < reqEnd && isWord(text[end])) end++;

  const ctx: Context = {
    position: "none", key: "", operator: "", partial: text.slice(start, caret),
    start, end, reqStart, reqEnd, keyEnd: start, listed: [], bare: text.slice(end, reqEnd).trim() === "",
  };
  const before = tokensOf(text, reqStart, start);
  const first = before[0];
  if (!first) return { ...ctx, position: "key" };
  if (before.length === 1 && first.text === "!") return { ...ctx, position: "notKey" };
  if (!first.word) return ctx;

  const rest = before.slice(1);
  const shape = rest.map((t) => t.text).join(" ");
  if (rest.length === 0) {
    // An operator is offered only where nothing follows the caret in the requirement: before `=sjc` a second
    // operator, or before a word its replacement, would leave text the API refuses or delete the word.
    if (text.slice(caret, reqEnd).trim() !== "") return ctx;
    return { ...ctx, position: "operator", key: first.text, keyEnd: first.end };
  }
  if (shape === "=" || shape === "= =" || shape === "! =") {
    return { ...ctx, position: "value", key: first.text, operator: shape === "=" ? "=" : shape === "= =" ? "==" : "!=" };
  }
  if ((rest[0]!.text === "in" || rest[0]!.text === "notin") && rest[0]!.word && rest[1]?.text === "(") {
    // Inside the list: items and commas alternate, and the caret follows the "(" or a comma.
    const items = rest.slice(2);
    const wellFormed = items.length % 2 === 0 && items.every((t, i) => (i % 2 === 0 ? t.word : t.text === ","));
    if (!wellFormed) return ctx;
    const listed = items.filter((t) => t.word).map((t) => t.text);
    for (const t of tokensOf(text, end, reqEnd)) {
      if (t.text === ")" && !t.word) break;
      if (t.word) listed.push(t.text);
    }
    return { ...ctx, position: "list", key: first.text, listed };
  }
  return ctx;
}

export type OptionKind = "key" | "operator" | "value" | "empty" | "exact" | "term" | "item";

export interface Option {
  kind: OptionKind;
  /** The text shown at the left of the option. */
  text: string;
  /** The muted note at the right, if any. */
  note?: string;
  key: string;
  /** The label value, for the value kinds. */
  value: string;
  /** What a pick writes in place of the token. */
  insert: string;
}

export interface Group {
  caption: string;
  options: Option[];
}

export interface Suggestions {
  groups: Group[];
  /** How many more options match than are in `groups`. */
  more: number;
  /** The line shown instead of options when typed text matches nothing. */
  none?: string;
}

const OPERATORS: readonly [string, string][] = [
  ["=", "is"],
  ["!=", "is not"],
  ["in (", "is one of"],
  ["notin (", "is none of"],
];

/** Ignoring case: those that start with `partial`, then those that contain it, each in the given order. */
function matching<T>(items: readonly T[], partial: string, textOf: (item: T) => string): T[] {
  if (partial === "") return [...items];
  const p = partial.toLowerCase();
  const starts: T[] = [];
  const contains: T[] = [];
  for (const item of items) {
    const t = textOf(item).toLowerCase();
    if (t.startsWith(p)) starts.push(item);
    else if (t.includes(p)) contains.push(item);
  }
  return [...starts, ...contains];
}

const shown = (value: string) => (value === "" ? "(empty)" : value);

function keyOption(k: IndexKey): Option {
  const sample = k.values.slice(0, SAMPLE_VALUES).map((v) => shown(v.value)).join(", ");
  const note = k.values.length > SAMPLE_VALUES ? `${sample}, …` : sample;
  return { kind: "key", text: k.key, note: note || undefined, key: k.key, value: "", insert: k.key };
}

/** Keeps the first `MAX_OPTIONS` options over all groups, in order, and counts the rest. */
function limited(groups: Group[], none?: string): Suggestions {
  let room = MAX_OPTIONS;
  let total = 0;
  const kept: Group[] = [];
  for (const g of groups) {
    total += g.options.length;
    const options = g.options.slice(0, room);
    room -= options.length;
    if (options.length > 0) kept.push({ caption: g.caption, options });
  }
  return { groups: kept, more: total - (MAX_OPTIONS - room), none: total === 0 ? none : undefined };
}

/** The options for a caret position, from `label_index` alone. */
export function suggest(ctx: Context, index: LabelIndex): Suggestions {
  const typed = ctx.partial;
  switch (ctx.position) {
    case "key":
    case "notKey": {
      const groups: Group[] = [{ caption: "Label keys", options: matching(index, typed, (k) => k.key).map(keyOption) }];
      // A typed value is offered as its whole term, so a newcomer need not know the key it is filed under.
      if (ctx.position === "key" && typed !== "" && ctx.bare) {
        const all = index.flatMap((k) => k.values.filter((v) => v.value !== "").map((v) => ({ k, v })));
        groups.push({
          caption: "Values",
          options: matching(all, typed, (x) => x.v.value).map(({ k, v }): Option =>
            v.selector === null
              ? { kind: "exact", text: `${k.key}=${v.value}`, note: "exact match", key: k.key, value: v.value, insert: "" }
              : { kind: "term", text: v.selector, key: k.key, value: v.value, insert: v.selector },
          ),
        });
      }
      return limited(groups, typed === "" ? undefined : `No label matches "${typed}".`);
    }
    case "operator": {
      const p = typed.toLowerCase();
      const options = OPERATORS.filter(([op]) => op.startsWith(p)).map(([op, words]): Option => ({ kind: "operator", text: op, note: words, key: ctx.key, value: "", insert: op }));
      return limited([{ caption: `Match ${ctx.key}`, options }]);
    }
    case "value": {
      const k = index.find((x) => x.key === ctx.key);
      if (!k) return { groups: [], more: 0 };
      const exactToo = ctx.operator !== "!=";
      const candidates = k.values.filter((v) => (v.value === "" ? typed === "" : v.selector !== null || exactToo));
      const options = matching(candidates, typed, (v) => v.value).map((v): Option => {
        if (v.value === "") return { kind: "empty", text: "(empty)", note: "empty value", key: k.key, value: "", insert: "" };
        if (v.selector === null) return { kind: "exact", text: v.value, note: "exact match", key: k.key, value: v.value, insert: "" };
        return { kind: "value", text: v.value, key: k.key, value: v.value, insert: v.value };
      });
      return limited([{ caption: `Values of ${k.key}`, options }], typed === "" ? undefined : `No value of ${k.key} matches "${typed}".`);
    }
    case "list": {
      const k = index.find((x) => x.key === ctx.key);
      if (!k) return { groups: [], more: 0 };
      const candidates = k.values.filter((v) => v.selector !== null && v.value !== "" && !ctx.listed.includes(v.value));
      const options = matching(candidates, typed, (v) => v.value).map((v): Option => ({ kind: "item", text: v.value, key: k.key, value: v.value, insert: v.value }));
      return limited([{ caption: `Values of ${k.key}`, options }], typed === "" ? undefined : `No value of ${k.key} matches "${typed}".`);
    }
    default:
      return { groups: [], more: 0 };
  }
}

export interface PickResult {
  text: string;
  caret: number;
  /** The pick applies the filter (a value, the empty value, a whole term). */
  apply: boolean;
  /** The list opens again for the new caret position (after a key or an operator). */
  reopen: boolean;
  /** The pick is an exact-match token; its requirement has left the text. */
  exact?: { key: string; value: string };
}

/** What picking `option` does to `text`. Only the token under the caret changes, or for an exact match its requirement. */
export function applyPick(text: string, ctx: Context, option: Option): PickResult {
  const replace = (from: number, to: number, insert: string) => ({ text: text.slice(0, from) + insert + text.slice(to), caret: from + insert.length });
  switch (option.kind) {
    case "key": {
      // `=` follows a key only where nothing follows it yet; after `!` nothing is added.
      const addEquals = ctx.position === "key" && ctx.bare;
      return { ...replace(ctx.start, ctx.end, option.insert + (addEquals ? "=" : "")), apply: false, reopen: addEquals };
    }
    case "operator": {
      // `=` and `!=` sit against the key; `in (` and `notin (` keep the space the user typed.
      const tight = option.insert === "=" || option.insert === "!=";
      return { ...replace(tight ? ctx.keyEnd : ctx.start, ctx.end, option.insert), apply: false, reopen: true };
    }
    case "exact": {
      // The requirement leaves the box with one adjoining comma.
      let from = ctx.reqStart;
      let to = ctx.reqEnd;
      if (to < text.length) to += 1;
      else if (from > 0) from -= 1;
      const head = text.slice(0, from).trim() === "" ? "" : text.slice(0, from);
      const tail = head === "" ? text.slice(to).replace(/^\s+/, "") : text.slice(to);
      return { text: head + tail, caret: head.length, apply: false, reopen: false, exact: { key: option.key, value: option.value } };
    }
    case "item": {
      // No comma and no `)` are added, except where another item follows with only spaces between
      // (`(ams,| sjc)`): there the comma is what keeps the two items apart.
      const next = text.slice(ctx.end, ctx.reqEnd).trimStart()[0];
      return { ...replace(ctx.start, ctx.end, option.insert + (isWord(next) ? "," : "")), apply: false, reopen: false };
    }
    default:
      return { ...replace(ctx.start, ctx.end, option.insert), apply: true, reopen: false };
  }
}
