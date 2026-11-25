// SPDX-License-Identifier: MIT
import { applyPick, contextAt, MAX_OPTIONS, placeholderFrom, suggest, type LabelIndex, type Option } from "./selectorSuggest";

const term = (key: string, value: string) => ({ value, selector: `${key}=${value}` });
const AWKWARD = "Team A (infra)";
const index: LabelIndex = [
  { key: "owner", values: [{ value: AWKWARD, selector: null }, term("owner", "plain")] },
  { key: "site", values: [term("site", "ams"), term("site", "sjc")] },
  { key: "site-code", values: [term("site-code", "x1")] },
  { key: "sitex", values: [term("sitex", "y")] },
  { key: "tier", values: [{ value: "", selector: "tier=" }, term("tier", "a"), term("tier", "B2"), term("tier", "ab"), { value: "has space", selector: null }] },
];

/** `"si|te=sjc"`: the text without the bar, and the caret where the bar was. */
const split = (marked: string) => ({ text: marked.replace("|", ""), caret: marked.indexOf("|") });
const at = (marked: string) => {
  const { text, caret } = split(marked);
  return contextAt(text, caret);
};
const found = (marked: string, idx: LabelIndex = index) => suggest(at(marked), idx);
const options = (marked: string, idx: LabelIndex = index) => found(marked, idx).groups.flatMap((g) => g.options);
const texts = (marked: string, idx: LabelIndex = index) => options(marked, idx).map((o) => o.text);
const captions = (marked: string) => found(marked).groups.map((g) => g.caption);
/** Picks the option with that text; returns the new text with the caret marked. */
const pick = (marked: string, optionText: string, idx: LabelIndex = index) => {
  const { text } = split(marked);
  const option = options(marked, idx).find((o) => o.text === optionText);
  if (!option) throw new Error(`no option ${optionText} in ${JSON.stringify(texts(marked, idx))}`);
  const r = applyPick(text, at(marked), option);
  return { ...r, marked: `${r.text.slice(0, r.caret)}|${r.text.slice(r.caret)}` };
};

describe("selectorSuggest", () => {
  it("UI-21 suggests keys at the start of a requirement, after a comma and after !", () => {
    const keys = ["owner", "site", "site-code", "sitex", "tier"];
    for (const marked of ["|", "  |", "site=sjc,|", "site=sjc, |", "site in (ams,sjc),|", "!|", "site=sjc,!|"]) {
      expect(captions(marked)).toEqual(["Label keys"]);
      expect(texts(marked)).toEqual(keys);
    }
    expect(at("|").position).toBe("key");
    expect(at("!ti|").position).toBe("notKey");
    expect(texts("ti|")).toEqual(["tier"]);
    expect(texts("site=sjc, ti|")).toEqual(["tier"]);
    expect(texts("!ti|")).toEqual(["tier"]);
    // A key carries a sample of its values: the first three, then ", …"; an empty value reads "(empty)".
    const notes = Object.fromEntries(options("|").map((o) => [o.text, o.note]));
    expect(notes).toEqual({ owner: `${AWKWARD}, plain`, site: "ams, sjc", "site-code": "x1", sitex: "y", tier: "(empty), a, B2, …" });
    // After a complete value and a space, or a closed list, there is nothing to suggest.
    for (const marked of ["site=sjc |", "site in (ams,sjc) |", "site in (ams) x|"]) expect(found(marked)).toEqual({ groups: [], more: 0 });
  });

  it("UI-21 typed text at the start of a requirement also lists the values that match, as whole terms", () => {
    expect(captions("sj|")).toEqual(["Values"]);
    expect(options("sj|")).toEqual([{ kind: "term", text: "site=sjc", key: "site", value: "sjc", insert: "site=sjc" }]);
    // Keys first, then values; a value is matched on the value, not on its key.
    expect(captions("a|")).toEqual(["Values"]);
    expect(texts("a|")).toEqual(["site=ams", "tier=a", "tier=ab", `owner=${AWKWARD}`, "owner=plain", "tier=has space"]);
    expect(captions("t|")).toEqual(["Label keys", "Values"]);
    expect(texts("t|")).toEqual(["tier", "site", "site-code", "sitex", `owner=${AWKWARD}`]);
    // A value with no term is written key=value and marked; the empty value is not listed here.
    expect(options("Team|")).toEqual([{ kind: "exact", text: `owner=${AWKWARD}`, note: "exact match", key: "owner", value: AWKWARD, insert: "" }]);
    expect(texts("tier|")).toEqual(["tier"]);
    // Not after `!`, and not where an operator already follows the token.
    expect(captions("!a|")).toEqual([]);
    expect(captions("sj|=x")).toEqual([]);
    // A whole term replaces the token and applies.
    expect(pick("size=l,sj|", "site=sjc")).toMatchObject({ marked: "size=l,site=sjc|", apply: true, reopen: false });
    // Nothing matches: a line instead of options.
    expect(found("zz|")).toEqual({ groups: [], more: 0, none: 'No label matches "zz".' });
    expect(found("!zz|").none).toBe('No label matches "zz".');
    expect(found("|").none).toBeUndefined();
  });

  it("UI-21 suggests the values of the key after =, == and !=, and nothing for a key that is not in the index", () => {
    for (const marked of ["site=|", "site==|", "site!=|", "site = |", "site != |", "tier=a,site=|"]) {
      expect(captions(marked)).toEqual(["Values of site"]);
      expect(texts(marked)).toEqual(["ams", "sjc"]);
    }
    expect(at("site==|")).toMatchObject({ position: "value", key: "site", operator: "==" });
    expect(at("site!=s|")).toMatchObject({ position: "value", key: "site", operator: "!=", partial: "s" });
    expect(texts("site=s|")).toEqual(["sjc", "ams"]);
    expect(options("site=|").every((o) => o.note === undefined)).toBe(true); // the term is not repeated beside the value
    for (const marked of ["nosuch=|", "nosuch=a|", "nosuch!=|", "nosuch in (|"]) expect(found(marked)).toEqual({ groups: [], more: 0 });
    expect(found("site=zz|")).toEqual({ groups: [], more: 0, none: 'No value of site matches "zz".' });
    // A value applies the filter when picked.
    expect(pick("site=|", "sjc")).toMatchObject({ marked: "site=sjc|", apply: true, reopen: false });
    expect(pick("site!=s|", "sjc")).toMatchObject({ marked: "site!=sjc|", apply: true });
  });

  it("UI-21 suggests operators after a key and a space", () => {
    expect(captions("site |")).toEqual(["Match site"]);
    expect(options("site |").map((o) => [o.text, o.note])).toEqual([["=", "is"], ["!=", "is not"], ["in (", "is one of"], ["notin (", "is none of"]]);
    expect(texts("site no|")).toEqual(["notin ("]);
    expect(texts("site i|")).toEqual(["in ("]);
    expect(texts("nosuch |")).toHaveLength(4); // operators need no index
    expect(texts("site x|")).toEqual([]);
    expect(found("site x|").none).toBeUndefined();
    expect(texts("site|")).toEqual(["site", "site-code", "sitex"]); // no space yet: still the key
    // An operator is inserted, applies nothing, and the values open.
    expect(pick("site |", "=")).toMatchObject({ marked: "site=|", apply: false, reopen: true });
    expect(pick("site |", "!=")).toMatchObject({ marked: "site!=|", apply: false, reopen: true });
    expect(pick("site no|", "notin (")).toMatchObject({ marked: "site notin (|", apply: false, reopen: true });
    expect(pick("tier=a,site i|", "in (").marked).toBe("tier=a,site in (|");
    expect(texts(pick("site |", "in (").marked)).toEqual(["ams", "sjc"]);
  });

  it("UI-21 inside an in or notin list suggests values that have a term, are not empty and are not in the list yet", () => {
    expect(captions("tier in (|")).toEqual(["Values of tier"]);
    expect(texts("tier in (|")).toEqual(["a", "B2", "ab"]); // not "", not "has space"
    expect(texts("tier notin (a, |")).toEqual(["B2", "ab"]);
    expect(texts("tier in (a,|,ab)")).toEqual(["B2"]); // items after the caret count too
    expect(texts("tier in (a, b|")).toEqual(["B2", "ab"]);
    expect(texts("site in (a|ms, sjc)")).toEqual(["ams"]); // the item under the caret is not "already listed"
    expect(found("tier in (zz|").none).toBe('No value of tier matches "zz".');
    // A pick replaces the item; no comma, no closing parenthesis, nothing applied.
    expect(pick("tier in (a, b|", "B2")).toMatchObject({ marked: "tier in (a, B2|", apply: false, reopen: false });
    expect(pick("site in (a|ms, sjc)", "ams").marked).toBe("site in (ams|, sjc)");
  });

  it("UI-21 a comma inside parentheses does not start a new requirement", () => {
    expect(at("site in (ams,|")).toMatchObject({ position: "list", key: "site", reqStart: 0 });
    expect(at("tier=a, site in (ams, s|")).toMatchObject({ position: "list", key: "site", partial: "s", reqStart: 7 });
    expect(at("site in (ams,sjc),ti|")).toMatchObject({ position: "key", partial: "ti", reqStart: 18 });
    // The requirement ends at the next top-level comma, not at one inside the list.
    expect(at("site in (a|ms,sjc),tier=a")).toMatchObject({ reqStart: 0, reqEnd: 17 });
  });

  it("UI-21 matches ignore case; prefix matches come before contains matches, each in the index's order", () => {
    expect(texts("tier=B|")).toEqual(["B2", "ab"]);
    expect(texts("tier=b|")).toEqual(["B2", "ab"]);
    expect(texts("tier=A|")).toEqual(["a", "ab", "has space"]);
    expect(texts("SITE|")).toEqual(["site", "site-code", "sitex"]);
    expect(texts("!e|")).toEqual(["owner", "site", "site-code", "sitex", "tier"]); // contains, in the index's order
    expect(texts("!s|")).toEqual(["site", "site-code", "sitex"]);
    expect(texts("!i|")).toEqual(["site", "site-code", "sitex", "tier"]);
    expect(texts("!o|")).toEqual(["owner", "site-code"]); // starts with, then contains
  });

  it("UI-21 renders at most 50 options and reports how many more match", () => {
    const many: LabelIndex = [{ key: "rack", values: Array.from({ length: 2000 }, (_, i) => term("rack", `r${String(i).padStart(4, "0")}`)) }];
    expect(options("rack=|", many)).toHaveLength(MAX_OPTIONS);
    expect(found("rack=|", many).more).toBe(1950);
    expect(found("rack=r00|", many).more).toBe(50);
    expect(found("rack=r000|", many)).toMatchObject({ more: 0 });
    expect(options("rack=r000|", many)).toHaveLength(10);
    // Keys are counted before values: 500 keys, each with a value that matches too.
    const wide: LabelIndex = Array.from({ length: 500 }, (_, i) => ({ key: `k${i}`, values: [term(`k${i}`, `k-value-${i}`)] }));
    const s = suggest(contextAt("k", 1), wide);
    expect(s.groups.map((g) => [g.caption, g.options.length])).toEqual([["Label keys", 50]]);
    expect(s.more).toBe(950);
    expect(suggest(contextAt("", 0), index).more).toBe(0);
  });

  it("UI-21 a pick replaces the whole token under the caret, also from the middle, and puts the caret after it", () => {
    // A key: `=` is added only where nothing follows the token.
    expect(pick("si|", "site")).toMatchObject({ marked: "site=|", apply: false, reopen: true });
    expect(pick("si|te=sjc", "sitex")).toMatchObject({ marked: "sitex|=sjc", apply: false, reopen: false });
    expect(pick("tier=a, si|", "site").marked).toBe("tier=a, site=|");
    expect(pick("si|,tier=a", "site").marked).toBe("site=|,tier=a");
    // After `!` the key is completed and nothing is added.
    expect(pick("!ti|", "tier")).toMatchObject({ marked: "!tier|", apply: false, reopen: false });
    // A value: the other requirements are untouched.
    expect(pick("site=s|jc,tier=a", "ams")).toMatchObject({ marked: "site=ams|,tier=a", apply: true });
    expect(pick("tier=a,site=|sjc", "ams").marked).toBe("tier=a,site=ams|");
    expect(pick("site==s|", "sjc").marked).toBe("site==sjc|");
    // A caret between two spaces: the operator goes against the key.
    expect(pick("site | ", "=").marked).toBe("site=| ");
  });

  it("UI-21 a key that is a prefix of another key is completed only when picked", () => {
    expect(texts("site|")).toEqual(["site", "site-code", "sitex"]);
    expect(texts("site=|")).toEqual(["ams", "sjc"]); // the values of `site` alone
    expect(texts("site-code=|")).toEqual(["x1"]);
    expect(pick("site|", "site").marked).toBe("site=|");
    expect(pick("site|", "site-code").marked).toBe("site-code=|");
  });

  it("UI-21 the empty value is offered as (empty) and leaves key=", () => {
    expect(options("tier=|")[0]).toEqual({ kind: "empty", text: "(empty)", note: "empty value", key: "tier", value: "", insert: "" });
    expect(texts("tier!=|")).toEqual(["(empty)", "a", "B2", "ab"]);
    expect(texts("tier=e|")).toEqual(["has space"]); // "(empty)" is not matched as text
    expect(pick("tier=|", "(empty)")).toMatchObject({ marked: "tier=|", apply: true, reopen: false });
    expect(pick("tier=|a,site=sjc", "(empty)")).toMatchObject({ marked: "tier=|,site=sjc", apply: true });
    expect(texts("tier in (|")).not.toContain("(empty)");
  });

  it("UI-22 a value with no term is offered only after = and ==, and its pick removes the requirement from the text", () => {
    const exact: Option = { kind: "exact", text: AWKWARD, note: "exact match", key: "owner", value: AWKWARD, insert: "" };
    expect(options("owner=|")).toEqual([exact, { kind: "value", text: "plain", key: "owner", value: "plain", insert: "plain" }]);
    expect(options("owner==T|")).toEqual([exact]);
    expect(texts("owner!=|")).toEqual(["plain"]);
    expect(texts("owner in (|")).toEqual(["plain"]);
    expect(texts("owner notin (|")).toEqual(["plain"]);
    const token = { key: "owner", value: AWKWARD };
    // The requirement goes with one adjoining comma; the box is not applied.
    expect(pick("owner=|", AWKWARD)).toEqual({ text: "", caret: 0, apply: false, reopen: false, exact: token, marked: "|" });
    expect(pick("owner=Te|", AWKWARD)).toMatchObject({ marked: "|", exact: token });
    expect(pick("site=sjc,owner=|", AWKWARD)).toMatchObject({ marked: "site=sjc|", apply: false, exact: token });
    expect(pick("site=sjc, owner==T|", AWKWARD)).toMatchObject({ marked: "site=sjc|", exact: token });
    expect(pick("owner=|, site=sjc", AWKWARD)).toMatchObject({ marked: "|site=sjc", exact: token });
    expect(pick("tier=a,owner=|,site=sjc", AWKWARD)).toMatchObject({ marked: "tier=a,|site=sjc", exact: token });
    expect(pick("site in (ams,sjc),owner=|", AWKWARD)).toMatchObject({ marked: "site in (ams,sjc)|", exact: token });
    expect(pick("site=sjc,Tea|", `owner=${AWKWARD}`)).toMatchObject({ marked: "site=sjc|", apply: false, exact: token });
  });

  it("UI-21 a pick never leaves two operators, deletes the word after the caret or joins two list items", () => {
    // An operator is offered only where nothing follows the caret in the requirement.
    expect(found("site |=sjc").groups).toEqual([]);
    expect(found("site |sjc").groups).toEqual([]);
    expect(found("site | sjc,tier=a").groups).toEqual([]);
    expect(texts("site | ")).toHaveLength(4);
    expect(texts("site |,tier=a")).toHaveLength(4); // the next requirement is not "what follows"
    // A list pick adds a comma only where another item follows with nothing but spaces between.
    expect(pick("tier in (a,| ab)", "B2").marked).toBe("tier in (a,B2,| ab)");
    expect(pick("tier in (a, B| ab)", "B2").marked).toBe("tier in (a, B2,| ab)");
    expect(pick("tier in (a,|,ab)", "B2").marked).toBe("tier in (a,B2|,ab)");
    expect(pick("tier in (a, |)", "B2").marked).toBe("tier in (a, B2|)");
    expect(pick("tier in (a, |", "B2").marked).toBe("tier in (a, B2|");
  });

  it("UI-9 in a narrow window the placeholder is the words alone, so it reads whole", () => {
    expect(placeholderFrom(index, true)).toBe("Type or pick a label");
    expect(placeholderFrom(index, false)).toBe("Type or pick a label, e.g. owner=plain");
    expect(placeholderFrom([], true)).toBeUndefined();
  });

  it("UI-21 awkward text never throws and never suggests for a shape it does not know", () => {
    for (const text of ["=", "==", "!=", "(", ")", ",", ",,", "!!", "a=(", "a b c", "in (", "site in", "site in (a b", "site=a=b", "site\tin\n(", "💥=", "a".repeat(63)]) {
      for (let caret = 0; caret <= text.length; caret++) expect(() => suggest(contextAt(text, caret), index)).not.toThrow();
    }
    expect(found("site in (a b|").groups).toEqual([]);
    expect(found("site=a=|").groups).toEqual([]);
    expect(found("=|").groups).toEqual([]);
    expect(texts("site\tin\n(|")).toEqual(["ams", "sjc"]); // tabs and newlines are spaces to the parser
    expect(contextAt("site", 99).partial).toBe("site"); // a caret past the end is the end
  });

  it("UI-9 the placeholder is the first term of the index that is at most 16 characters, after the words Type or pick a label", () => {
    expect(placeholderFrom(index)).toBe("Type or pick a label, e.g. owner=plain");
    expect(placeholderFrom([{ key: "a", values: [{ value: "x y", selector: null }, term("a", "z")] }, { key: "b", values: [term("b", "1")] }])).toBe("Type or pick a label, e.g. a=z");
    expect(placeholderFrom([{ key: "a-rather-long-label-key-name", values: [term("a-rather-long-label-key-name", "v")] }, { key: "size", values: [term("size", "a-16-chars-")] }, { key: "t", values: [term("t", "1")] }])).toBe(
      "Type or pick a label, e.g. size=a-16-chars-",
    );
    // No term short enough, or none at all: the words alone. An empty index: no placeholder.
    expect(placeholderFrom([{ key: "a-rather-long-label-key-name", values: [term("a-rather-long-label-key-name", "v")] }])).toBe("Type or pick a label");
    expect(placeholderFrom([{ key: "a", values: [{ value: "x y", selector: null }] }])).toBe("Type or pick a label");
    expect(placeholderFrom([])).toBeUndefined();
  });
});
