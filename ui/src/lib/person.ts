// SPDX-License-Identifier: MIT
// How a person is shown (UX-REVIEW A1.1). One rule for every page: the full name, else the username, else the email,
// never blank. The API sends its own display value (`display_name`, or `holder.name` and `actor.name`, which already
// follow the same rule); this helper only guards against a value that is empty or only spaces, so an older API or an
// unnamed person never renders as a blank cell, heading or accessible name.

export interface PersonLike {
  display_name?: string | null;
  name?: string | null;
  username?: string | null;
  email?: string | null;
}

const clean = (v: string | null | undefined): string => (v ?? "").trim();

/** The name to show for a person. A value that is only spaces counts as no value. */
export function displayName(p: PersonLike): string {
  return clean(p.display_name) || clean(p.name) || clean(p.username) || clean(p.email);
}

/** A person known only by the display value an API sent (a holder, an actor). */
export function displayNameOf(value: string | null | undefined): string {
  return displayName({ display_name: value });
}

/**
 * Labels for a list of people sorted by display name (case-insensitive, then id). When two or more share a display
 * name, each of those reads `{name} · {email}` so they can be told apart. Admin lists only: a developer page never
 * adds an email.
 */
export function personOptions<T extends PersonLike & { id: number }>(people: readonly T[]): { id: number; label: string }[] {
  const names = people.map((p) => displayName(p));
  const counts = new Map<string, number>();
  for (const n of names) counts.set(n.toLowerCase(), (counts.get(n.toLowerCase()) ?? 0) + 1);
  return people
    .map((p, i) => {
      const n = names[i]!;
      const email = clean(p.email);
      return { id: p.id, name: n, label: (counts.get(n.toLowerCase()) ?? 0) > 1 && email ? `${n} · ${email}` : n };
    })
    .sort((a, b) => a.name.localeCompare(b.name, undefined, { sensitivity: "base" }) || a.id - b.id)
    .map(({ id, label }) => ({ id, label }));
}
