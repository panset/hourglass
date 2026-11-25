// SPDX-License-Identifier: MIT
// The Label selector box (UI-21, UI-22): a combobox with a list popup. Focus stays in the input; the active
// option is named by `aria-activedescendant`. No option is active until the user moves into the list, so Enter
// on typed text submits the form exactly as a plain text box does. With an empty `label_index` (also an API
// that sends none) it is a plain text box.
import {
  forwardRef,
  useEffect,
  useId,
  useImperativeHandle,
  useLayoutEffect,
  useMemo,
  useRef,
  useState,
  type ChangeEvent,
  type KeyboardEvent,
  type MouseEvent,
} from "react";
import { Field, Input } from "../../components";
import { applyPick, contextAt, placeholderFrom, suggest, type LabelIndex, type Option } from "./selectorSuggest";
import styles from "./setups.module.css";

export const SELECTOR_HELP =
  "Forms: key=value, key!=value, key in (a,b), key notin (a,b), key, !key. Commas mean AND. The table still shows the last filter that worked.";

export interface SelectorBoxProps {
  value: string;
  onChange: (text: string) => void;
  /** A pick that filters at once: the whole text of the box, as it is after the pick. */
  onApply: (text: string) => void;
  /** A picked value that no selector can express: an exact-match token (UI-22). */
  onExact: (key: string, value: string) => void;
  /** Text for the page's one status line. */
  onAnnounce: (text: string) => void;
  labelIndex: LabelIndex;
  /** The API's message for a refused selector. */
  error?: string;
  /** Told when the suggestion list opens or closes. */
  onListOpen?: (open: boolean) => void;
}

/** Below this width the box is too narrow for the placeholder's example (measured at 390 px). */
const NARROW = "(max-width: 480px)";
function useNarrow(): boolean {
  const query = () => (typeof window.matchMedia === "function" ? window.matchMedia(NARROW) : undefined);
  const [narrow, setNarrow] = useState(() => query()?.matches ?? false);
  useEffect(() => {
    const q = query();
    if (!q) return;
    const changed = () => setNarrow(q.matches);
    q.addEventListener?.("change", changed);
    return () => q.removeEventListener?.("change", changed);
  }, []);
  return narrow;
}

export interface SelectorBoxHandle {
  focus: () => void;
}

const CARET_KEYS = new Set(["ArrowLeft", "ArrowRight", "Home", "End"]);
const keepFocus = (e: MouseEvent) => e.preventDefault();

export const SelectorBox = forwardRef<SelectorBoxHandle, SelectorBoxProps>(function SelectorBox(
  { value, onChange, onApply, onExact, onAnnounce, labelIndex, error, onListOpen },
  ref,
) {
  const baseId = useId();
  const listId = `${baseId}-list`;
  const wrapRef = useRef<HTMLDivElement>(null);
  const input = () => wrapRef.current?.querySelector("input") ?? null;
  useImperativeHandle(ref, () => ({ focus: () => input()?.focus() }));

  const combo = labelIndex.length > 0;
  const [open, setOpen] = useState(false);
  // The text and caret the list was computed for. Not updated during composition.
  const [at, setAt] = useState({ text: "", caret: 0 });
  const [active, setActive] = useState(-1);
  const composing = useRef(false);
  const caretAfterPick = useRef<number | null>(null);

  const context = useMemo(() => contextAt(at.text, at.caret), [at]);
  const found = useMemo(() => suggest(context, labelIndex), [context, labelIndex]);
  const options = useMemo(() => found.groups.flatMap((g) => g.options), [found]);
  const shown = combo && open && (options.length > 0 || found.none !== undefined);
  const optionId = (i: number) => `${baseId}-opt-${i}`;
  const activeId = shown && active >= 0 && active < options.length ? optionId(active) : undefined;

  // The caret goes after the inserted text once React has written the new value.
  useLayoutEffect(() => {
    const c = caretAfterPick.current;
    if (c === null) return;
    caretAfterPick.current = null;
    input()?.setSelectionRange(c, c);
  });

  useEffect(() => {
    if (activeId) document.getElementById(activeId)?.scrollIntoView?.({ block: "nearest" });
  }, [activeId]);

  // A tap outside closes the list where a tap does not blur the box.
  useEffect(() => {
    if (!shown) return;
    const outside = (e: Event) => {
      if (!wrapRef.current?.contains(e.target as Node)) setOpen(false);
    };
    document.addEventListener("pointerdown", outside);
    return () => document.removeEventListener("pointerdown", outside);
  }, [shown]);

  const narrow = useNarrow();
  const listOpen = useRef(onListOpen);
  listOpen.current = onListOpen;
  useEffect(() => {
    listOpen.current?.(shown);
  }, [shown]);

  const total = options.length + found.more;
  const content = shown ? `${options.map((o) => `${o.kind} ${o.text}`).join("\n")}\n${found.more}` : null;
  const announce = useRef(onAnnounce);
  announce.current = onAnnounce;
  useEffect(() => {
    if (content === null) return;
    announce.current(total === 0 ? "No suggestions." : total === 1 ? "1 suggestion." : `${total} suggestions.`);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [content]);

  /** Reads the box's text and caret, and returns the options for them. */
  const sync = (el: HTMLInputElement) => {
    const next = { text: el.value, caret: el.selectionStart ?? el.value.length };
    setAt(next);
    setActive(-1);
    return suggest(contextAt(next.text, next.caret), labelIndex).groups.flatMap((g) => g.options);
  };

  const pick = (option: Option) => {
    // During composition the list is not the list of the text in the box: leave both alone.
    if (at.text !== value) return setOpen(false);
    const result = applyPick(value, context, option);
    onChange(result.text);
    caretAfterPick.current = result.caret;
    setAt({ text: result.text, caret: result.caret });
    setActive(-1);
    setOpen(result.reopen);
    if (result.exact) onExact(result.exact.key, result.exact.value);
    if (result.apply) onApply(result.text);
    input()?.focus();
  };

  const onInput = (e: ChangeEvent<HTMLInputElement>) => {
    onChange(e.target.value);
    if (!combo || composing.current) return;
    sync(e.target);
    setOpen(true);
  };

  const onKeyDown = (e: KeyboardEvent<HTMLInputElement>) => {
    if (composing.current || e.nativeEvent.isComposing) {
      // The input method owns these keys. Enter confirms its text; it must not also submit the form.
      if (e.key === "Enter") e.preventDefault();
      return;
    }
    if (!combo) return;
    switch (e.key) {
      case "ArrowDown":
      case "ArrowUp": {
        e.preventDefault();
        const down = e.key === "ArrowDown";
        if (!shown) {
          const n = sync(e.currentTarget).length;
          setOpen(true);
          setActive(n === 0 ? -1 : down ? 0 : n - 1);
        } else if (options.length > 0) {
          const n = options.length;
          setActive((a) => (down ? (a + 1) % n : a <= 0 ? n - 1 : a - 1));
        }
        break;
      }
      case "Enter":
        if (shown && active >= 0 && options[active]) {
          e.preventDefault(); // a pick, not a submit
          pick(options[active]);
        } else {
          setOpen(false); // the form submits the text as typed
        }
        break;
      case "Escape":
        if (shown) {
          e.preventDefault();
          e.stopPropagation();
          setOpen(false); // the text stays
        }
        break;
      case "Tab":
        setOpen(false);
        break;
    }
  };

  const toggle = () => {
    const el = input();
    if (!el) return;
    el.focus();
    if (shown) return setOpen(false);
    sync(el);
    setOpen(true);
  };

  const errorId = `${baseId}-error`;
  const helpId = `${baseId}-help`;

  return (
    <div className={styles.selectorField}>
      <Field label="Label selector">
        {(p) => {
          let n = 0; // option index over all groups
          return (
          <>
            <div className={styles.selectorWrap} ref={wrapRef}>
              <Input
                {...p}
                {...(combo
                  ? { role: "combobox", "aria-autocomplete": "list", "aria-expanded": shown, "aria-controls": listId, "aria-activedescendant": activeId }
                  : {})}
                className={styles.selectorInput}
                placeholder={placeholderFrom(labelIndex, narrow)}
                autoComplete="off"
                spellCheck={false}
                value={value}
                aria-invalid={error ? true : undefined}
                aria-describedby={error ? `${errorId} ${helpId}` : undefined}
                onChange={onInput}
                onKeyDown={onKeyDown}
                onKeyUp={(e) => {
                  if (open && CARET_KEYS.has(e.key) && !composing.current) sync(e.currentTarget);
                }}
                onClick={(e) => {
                  if (!combo) return;
                  sync(e.currentTarget);
                  setOpen(true);
                }}
                onBlur={() => setOpen(false)}
                onCompositionStart={() => {
                  composing.current = true;
                }}
                onCompositionEnd={(e) => {
                  composing.current = false;
                  if (!combo) return;
                  sync(e.currentTarget);
                  setOpen(true);
                }}
              />
              {combo ? (
                <button
                  type="button"
                  tabIndex={-1}
                  className={styles.toggle}
                  aria-label="Show label suggestions"
                  aria-expanded={shown}
                  aria-controls={listId}
                  onMouseDown={keepFocus}
                  onClick={toggle}
                />
              ) : null}
              {shown ? (
                <div id={listId} role="listbox" aria-label="Label suggestions" className={styles.list} onMouseDown={keepFocus}>
                  <div className={styles.listScroll}>
                    {found.groups.map((g, gi) => (
                      <div key={g.caption} role="group" aria-labelledby={`${baseId}-cap-${gi}`}>
                        <div id={`${baseId}-cap-${gi}`} role="presentation" className={styles.caption}>
                          {g.caption}
                        </div>
                        {g.options.map((o) => {
                          const i = n++;
                          return (
                            <div
                              key={`${o.kind} ${o.text}`}
                              id={optionId(i)}
                              role="option"
                              aria-selected={i === active}
                              aria-label={o.note ? `${o.text}, ${o.kind === "key" ? "values: " : ""}${o.note}` : undefined}
                              className={styles.option}
                              onClick={() => pick(o)}
                            >
                              <span className={styles.optMain}>{o.text}</span>
                              {o.note ? (
                                <span className={styles.optNote}>{o.note}</span>
                              ) : null}
                            </div>
                          );
                        })}
                      </div>
                    ))}
                    {found.none !== undefined ? (
                      <div role="presentation" className={styles.listNote}>
                        {found.none}
                      </div>
                    ) : null}
                  </div>
                  {found.more > 0 ? (
                    <div role="presentation" className={styles.more}>
                      {found.more} more. Keep typing to narrow.
                    </div>
                  ) : null}
                </div>
              ) : null}
            </div>
            {error ? (
              <>
                <div id={errorId} className={styles.selectorError}>
                  {error}
                </div>
                <div id={helpId} className={styles.selectorHelp}>
                  {SELECTOR_HELP}
                </div>
              </>
            ) : null}
          </>
          );
        }}
      </Field>
    </div>
  );
});
