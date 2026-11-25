// SPDX-License-Identifier: MIT
import { displayName, displayNameOf, personOptions } from "./person";

describe("A1.1 displayName: name, else username, else email, never blank when any is set", () => {
  it("prefers the API's display_name", () => {
    expect(displayName({ display_name: "Ada Lovelace", name: "x", username: "ada", email: "ada@example.com" })).toBe("Ada Lovelace");
  });
  it("falls back from name to username to email", () => {
    expect(displayName({ name: "Ada Lovelace", username: "ada", email: "ada@example.com" })).toBe("Ada Lovelace");
    expect(displayName({ name: "", username: "ada", email: "ada@example.com" })).toBe("ada");
    expect(displayName({ name: "", username: "", email: "ada@example.com" })).toBe("ada@example.com");
  });
  it("counts a value of only spaces as no value and trims the ends", () => {
    expect(displayName({ display_name: "   ", name: " \t", username: "ada", email: "ada@example.com" })).toBe("ada");
    expect(displayName({ display_name: "  ", name: "", username: " ", email: "ada@example.com" })).toBe("ada@example.com");
    expect(displayName({ name: "  Ada  " })).toBe("Ada");
  });
  it("is empty only when nothing at all is known", () => {
    expect(displayName({})).toBe("");
    expect(displayNameOf(null)).toBe("");
    expect(displayNameOf("Sam Lee")).toBe("Sam Lee");
  });
});

describe("A1.1 personOptions: sorted by display name, duplicates told apart by email", () => {
  it("sorts ignoring case, then by id, and a nameless user sorts under its fallback", () => {
    const out = personOptions([
      { id: 3, display_name: "zoe" },
      { id: 2, display_name: "", name: "", username: "", email: "mia@example.com" },
      { id: 1, display_name: "Aaron" },
      { id: 4, display_name: "Zoe", email: "z@example.com" },
    ]);
    expect(out.map((o) => o.id)).toEqual([1, 2, 3, 4]);
  });
  it("shows name · email for two people with the same name, and only for them", () => {
    const out = personOptions([
      { id: 1, display_name: "Sam Lee", email: "sam1@example.com" },
      { id: 2, display_name: "Sam Lee", email: "sam2@example.com" },
      { id: 3, display_name: "Priya N.", email: "priya@example.com" },
    ]);
    expect(out.map((o) => o.label)).toEqual(["Priya N.", "Sam Lee · sam1@example.com", "Sam Lee · sam2@example.com"]);
  });
});
