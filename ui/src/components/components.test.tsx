// SPDX-License-Identifier: MIT
import { act, fireEvent, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useRef, useState } from "react";
import { Alert, Badge, BrandMark, Button, Chip, ConfirmDialog, Field, Input, LinkButton, Modal, Select, Table, ToastProvider, TOAST_MS, useToast } from ".";

describe("Button", () => {
  it("is a real button that does not submit by default", async () => {
    const onClick = vi.fn();
    render(<Button onClick={onClick}>Release</Button>);
    const b = screen.getByRole("button", { name: "Release" });
    expect(b).toHaveAttribute("type", "button");
    await userEvent.click(b);
    expect(onClick).toHaveBeenCalled();
  });
  it("LinkButton is a link", () => {
    render(<LinkButton href="/auth/oidc/start">Sign in with SSO</LinkButton>);
    expect(screen.getByRole("link", { name: "Sign in with SSO" })).toHaveAttribute("href", "/auth/oidc/start");
  });
});

describe("BrandMark", () => {
  it("draws the seven-bar mark in the text colour and is hidden from assistive technology", () => {
    render(<BrandMark />);
    const mark = screen.getByTestId("brand-mark");
    expect(mark).toHaveAttribute("aria-hidden", "true");
    expect(mark).toHaveAttribute("fill", "currentColor");
    expect(mark.querySelectorAll("rect")).toHaveLength(7);
  });
});

describe("Badge and Chip", () => {
  it("render their text", () => {
    render(
      <>
        <Badge tone="blue">Deployed</Badge>
        <Chip>size=medium</Chip>
      </>,
    );
    expect(screen.getByText("Deployed")).toBeInTheDocument();
    expect(screen.getByText("size=medium").tagName).toBe("CODE");
  });
});

describe("Table", () => {
  const cols = [
    { key: "name", header: "Setup", render: (r: { name: string }) => r.name },
    { key: "act", header: "", render: () => <button>Go</button> },
  ];
  it("renders rows under an accessible name, with a named action column", () => {
    render(<Table label="Setups" columns={cols} rows={[{ name: "setup-01" }]} rowKey={(r) => r.name} />);
    expect(screen.getByRole("table", { name: "Setups" })).toBeInTheDocument();
    expect(screen.getByRole("columnheader", { name: "Actions" })).toBeInTheDocument();
    expect(screen.getByRole("cell", { name: "setup-01" })).toBeInTheDocument();
  });
  it("shows the empty state", () => {
    render(<Table label="Holds" columns={cols} rows={[]} rowKey={(r: { name: string }) => r.name} emptyMessage="No holds." />);
    expect(screen.getByText("No holds.")).toBeInTheDocument();
  });
});

describe("Field", () => {
  it("ties label, hint and error to the control", () => {
    render(
      <Field label="Reason" hint="Required" error="Reason is required">
        {(p) => <Input {...p} />}
      </Field>,
    );
    const input = screen.getByLabelText("Reason");
    expect(input).toHaveAttribute("aria-invalid", "true");
    expect(input).toHaveAccessibleDescription("Reason is required Required");
  });
  it("works with Select", () => {
    render(
      <Field label="State">
        {(p) => (
          <Select {...p}>
            <option>All</option>
          </Select>
        )}
      </Field>,
    );
    expect(screen.getByRole("combobox", { name: "State" })).toBeInTheDocument();
  });
});

describe("Alert", () => {
  it("errors use role alert", () => {
    render(<Alert tone="error">Bad</Alert>);
    expect(screen.getByRole("alert")).toHaveTextContent("Bad");
  });
});

describe("Modal and ConfirmDialog", () => {
  function Harness({ onConfirm = () => {} }: { onConfirm?: () => void }) {
    const [open, setOpen] = useState(false);
    return (
      <>
        <button onClick={() => setOpen(true)}>Acquire</button>
        <ConfirmDialog open={open} title="Acquire setup-05?" confirmLabel="Acquire now" onConfirm={onConfirm} onCancel={() => setOpen(false)}>
          Costs 2.0 tokens / hour.
        </ConfirmDialog>
      </>
    );
  }

  it("is labelled, moves focus in, closes on Escape and returns focus", async () => {
    render(<Harness />);
    const opener = screen.getByRole("button", { name: "Acquire" });
    await userEvent.click(opener);
    const dialog = screen.getByRole("dialog", { name: "Acquire setup-05?" });
    expect(dialog).toHaveAttribute("aria-modal", "true");
    expect(dialog).toContainElement(document.activeElement as HTMLElement);
    await userEvent.keyboard("{Escape}");
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    expect(opener).toHaveFocus();
  });

  it("keeps Tab inside the dialog", async () => {
    render(<Harness />);
    await userEvent.click(screen.getByRole("button", { name: "Acquire" }));
    const confirm = screen.getByRole("button", { name: "Acquire now" });
    const cancel = screen.getByRole("button", { name: "Cancel" });
    expect(cancel).toHaveFocus(); // the safe button, so a stray Enter does not confirm
    await userEvent.tab(); // wraps to the first control: the x in the header
    expect(screen.getByRole("button", { name: "Close" })).toHaveFocus();
    await userEvent.tab();
    expect(confirm).toHaveFocus();
    await userEvent.tab();
    expect(cancel).toHaveFocus();
    await userEvent.tab({ shift: true });
    expect(confirm).toHaveFocus();
  });

  it("confirm and cancel call back; errors and busy show", async () => {
    const onConfirm = vi.fn();
    const { rerender } = render(
      <ConfirmDialog open title="T" confirmLabel="Do it" onConfirm={onConfirm} onCancel={() => {}} error="Refused">
        body
      </ConfirmDialog>,
    );
    await userEvent.click(screen.getByRole("button", { name: "Do it" }));
    expect(onConfirm).toHaveBeenCalled();
    expect(screen.getByRole("alert")).toHaveTextContent("Refused");
    rerender(
      <ConfirmDialog open title="T" confirmLabel="Do it" busy onConfirm={onConfirm} onCancel={() => {}}>
        body
      </ConfirmDialog>,
    );
    expect(screen.getByRole("button", { name: "Do it" })).toBeDisabled();
  });

  // The dialog has two buttons named Close: the x in the header first, the footer's last.
  const footerClose = () => within(screen.getByRole("dialog")).getAllByRole("button", { name: "Close" }).at(-1)!;
  const headerClose = () => within(screen.getByRole("dialog")).getAllByRole("button", { name: "Close" })[0]!;

  function Panel({ withFallback = true }: { withFallback?: boolean }) {
    const [open, setOpen] = useState(false);
    const [rowShown, setRowShown] = useState(true);
    const heading = useRef<HTMLHeadingElement>(null);
    return (
      <>
        <h1 ref={heading} tabIndex={-1}>
          Setups
        </h1>
        {rowShown ? <button onClick={() => setOpen(true)}>Details for setup-01</button> : null}
        <Modal
          open={open}
          title="Details: setup-01"
          onClose={() => setOpen(false)}
          fallbackFocus={withFallback ? heading : undefined}
          footer={
            <>
              <button onClick={() => setRowShown(false)}>Hide the row</button>
              <button onClick={() => setOpen(false)}>Close</button>
            </>
          }
        >
          {/* No control in the body, so the marked heading takes focus on open. */}
          <h3 tabIndex={-1} data-autofocus="">
            Price
          </h3>
          <p>Weight 2</p>
        </Modal>
      </>
    );
  }

  it("Modal keeps Shift+Tab inside when a heading has focus", async () => {
    render(<Panel />);
    await userEvent.click(screen.getByRole("button", { name: "Details for setup-01" }));
    expect(screen.getByRole("heading", { name: "Price" })).toHaveFocus();
    await userEvent.tab({ shift: true });
    expect(footerClose()).toHaveFocus();
    expect(screen.getByRole("dialog")).toContainElement(document.activeElement as HTMLElement);
  });

  it("Modal returns focus to the fallback when the opener is gone", async () => {
    render(<Panel />);
    await userEvent.click(screen.getByRole("button", { name: "Details for setup-01" }));
    await userEvent.click(screen.getByRole("button", { name: "Hide the row" }));
    expect(screen.queryByRole("button", { name: "Details for setup-01" })).not.toBeInTheDocument();
    await userEvent.keyboard("{Escape}");
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "Setups" })).toHaveFocus();
  });

  it("Modal returns focus to the opener, not the fallback, when the opener is still there", async () => {
    render(<Panel />);
    const opener = screen.getByRole("button", { name: "Details for setup-01" });
    await userEvent.click(opener);
    await userEvent.click(footerClose());
    expect(opener).toHaveFocus();
  });

  it("Modal without a fallback leaves focus alone when the opener is gone, as before", async () => {
    render(<Panel withFallback={false} />);
    await userEvent.click(screen.getByRole("button", { name: "Details for setup-01" }));
    await userEvent.click(screen.getByRole("button", { name: "Hide the row" }));
    await userEvent.keyboard("{Escape}");
    expect(screen.getByRole("heading", { name: "Setups" })).not.toHaveFocus();
  });

  // A2.7 acceptance checks. Numbers refer to .orchestrator/ext-feedback-names-modal/tasks/02-ux-review/UX-REVIEW.md.
  const backdropOf = () => screen.getByRole("dialog").parentElement as HTMLElement;

  it("A2.7-1/2 the dialog is a header with the x, a body that scrolls, and a footer outside the body", async () => {
    render(<Panel />);
    await userEvent.click(screen.getByRole("button", { name: "Details for setup-01" }));
    const dialog = screen.getByRole("dialog");
    const [header, body, footer] = Array.from(dialog.children) as HTMLElement[];
    expect(header).toContainElement(screen.getByRole("heading", { name: "Details: setup-01" }));
    expect(header).toContainElement(headerClose());
    expect(body).toHaveTextContent("Weight 2");
    expect(body).not.toContainElement(footerClose());
    expect(footer).toContainElement(footerClose());
    expect(headerClose().tagName).toBe("BUTTON");
    expect(headerClose()).toHaveAttribute("type", "button");
  });

  it("A2.7-3 a click on the backdrop closes a read-only dialog and focus returns to the opener", async () => {
    render(<Panel />);
    const opener = screen.getByRole("button", { name: "Details for setup-01" });
    await userEvent.click(opener);
    await userEvent.click(backdropOf());
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    expect(opener).toHaveFocus();
  });

  it("A2.7-4 Escape closes and focus returns to the opener", async () => {
    render(<Panel />);
    const opener = screen.getByRole("button", { name: "Details for setup-01" });
    await userEvent.click(opener);
    await userEvent.keyboard("{Escape}");
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    expect(opener).toHaveFocus();
  });

  it("A2.7-5 the x is named Close, closes the dialog, and Shift+Tab from the footer Close reaches it", async () => {
    render(<Panel />);
    const opener = screen.getByRole("button", { name: "Details for setup-01" });
    await userEvent.click(opener);
    // Tab order: the x, then the footer controls (Hide the row, Close).
    footerClose().focus();
    await userEvent.tab({ shift: true });
    await userEvent.tab({ shift: true });
    expect(headerClose()).toHaveFocus();
    await userEvent.click(headerClose());
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    expect(opener).toHaveFocus();
  });

  it("A2.7-6 opening still puts focus on the marked heading, not on the x", async () => {
    render(<Panel />);
    await userEvent.click(screen.getByRole("button", { name: "Details for setup-01" }));
    expect(screen.getByRole("heading", { name: "Price" })).toHaveFocus();
    expect(headerClose()).not.toHaveFocus();
  });

  it("A2.7-6 a confirmation still opens on Cancel", async () => {
    render(<Harness />);
    await userEvent.click(screen.getByRole("button", { name: "Acquire" }));
    expect(screen.getByRole("button", { name: "Cancel" })).toHaveFocus();
  });

  it("F3 a press that starts on the backdrop and is released inside the dialog does not close it", async () => {
    render(<Panel />);
    await userEvent.click(screen.getByRole("button", { name: "Details for setup-01" }));
    const backdrop = backdropOf();
    fireEvent.mouseDown(backdrop);
    fireEvent.mouseUp(screen.getByText("Weight 2"));
    fireEvent.click(backdrop);
    expect(screen.getByRole("dialog")).toBeInTheDocument();
  });

  it("A2.7-7 a press that starts inside the dialog and ends on the backdrop does not close it", async () => {
    render(<Panel />);
    await userEvent.click(screen.getByRole("button", { name: "Details for setup-01" }));
    const backdrop = backdropOf();
    // What a browser sends for a text selection dragged out of the dialog: the click lands on the
    // common ancestor, which is the backdrop.
    fireEvent.mouseDown(screen.getByText("Weight 2"));
    fireEvent.mouseUp(backdrop);
    fireEvent.click(backdrop);
    expect(screen.getByRole("dialog")).toBeInTheDocument();
    // A later clean press and release on the backdrop still closes it.
    await userEvent.click(backdrop);
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });

  it("A2.7-8 the page behind does not scroll while a dialog is open, and is restored exactly on close", async () => {
    const html = document.documentElement;
    html.style.overflow = "auto";
    html.style.paddingRight = "3px";
    const { unmount } = render(<Panel />);
    await userEvent.click(screen.getByRole("button", { name: "Details for setup-01" }));
    expect(html.style.overflow).toBe("hidden");
    await userEvent.keyboard("{Escape}");
    expect(html.style.overflow).toBe("auto");
    expect(html.style.paddingRight).toBe("3px");
    // Unmounting an open dialog (a page change) restores it as well.
    await userEvent.click(screen.getByRole("button", { name: "Details for setup-01" }));
    expect(html.style.overflow).toBe("hidden");
    unmount();
    expect(html.style.overflow).toBe("auto");
    expect(html.style.paddingRight).toBe("3px");
    html.style.overflow = "";
    html.style.paddingRight = "";
  });

  it("A2.7-8 two open dialogs keep the lock until the last one closes", () => {
    const html = document.documentElement;
    const a = render(<Modal open title="A" onClose={() => {}}>a</Modal>);
    const b = render(<Modal open title="B" onClose={() => {}}>b</Modal>);
    expect(html.style.overflow).toBe("hidden");
    a.unmount();
    expect(html.style.overflow).toBe("hidden");
    b.unmount();
    expect(html.style.overflow).toBe("");
  });

  function FormDialog() {
    const [open, setOpen] = useState(true);
    const [text, setText] = useState("");
    return (
      <Modal open={open} title="Create service account" onClose={() => setOpen(false)} dismissOnBackdrop={false}>
        <input aria-label="Name" value={text} onChange={(e) => setText(e.target.value)} />
      </Modal>
    );
  }

  it("A2.7-9 a form dialog ignores a backdrop click and keeps its text; Escape and the x close it", async () => {
    const { unmount } = render(<FormDialog />);
    await userEvent.type(screen.getByRole("textbox", { name: "Name" }), "billing");
    await userEvent.click(backdropOf());
    expect(screen.getByRole("textbox", { name: "Name" })).toHaveValue("billing");
    await userEvent.keyboard("{Escape}");
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    unmount();
    render(<FormDialog />);
    await userEvent.click(headerClose());
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });

  it("A2.7-11 a confirmation closes on a backdrop click like Cancel and focus returns to its opener", async () => {
    render(<Harness />);
    const opener = screen.getByRole("button", { name: "Acquire" });
    await userEvent.click(opener);
    await userEvent.click(backdropOf());
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    expect(opener).toHaveFocus();
  });

  it("A2.7-11 B1.1 while busy, Escape, the x and the backdrop do nothing", async () => {
    const onCancel = vi.fn();
    render(
      <ConfirmDialog open title="Acquire?" confirmLabel="Acquire now" busy onConfirm={() => {}} onCancel={onCancel}>
        body
      </ConfirmDialog>,
    );
    expect(headerClose()).toBeDisabled();
    await userEvent.keyboard("{Escape}");
    await userEvent.click(headerClose());
    await userEvent.click(backdropOf());
    expect(onCancel).not.toHaveBeenCalled();
    expect(screen.getByRole("dialog")).toBeInTheDocument();
  });

  it("A2.7-11 a Modal with busy set behaves the same", async () => {
    const onClose = vi.fn();
    render(
      <Modal open title="Saving" busy onClose={onClose}>
        body
      </Modal>,
    );
    await userEvent.keyboard("{Escape}");
    await userEvent.click(backdropOf());
    expect(onClose).not.toHaveBeenCalled();
  });

  it("A2.7-12 a confirmation that holds a field can opt out of the backdrop", async () => {
    const onCancel = vi.fn();
    render(
      <ConfirmDialog open title="Force-release?" confirmLabel="Go" dismissOnBackdrop={false} onConfirm={() => {}} onCancel={onCancel}>
        <input aria-label="Reason" defaultValue="why" />
      </ConfirmDialog>,
    );
    await userEvent.click(backdropOf());
    expect(onCancel).not.toHaveBeenCalled();
    expect(screen.getByRole("textbox", { name: "Reason" })).toHaveValue("why");
    await userEvent.click(headerClose());
    expect(onCancel).toHaveBeenCalledTimes(1);
  });

  it("Escape during an IME composition does not close the dialog", async () => {
    const onClose = vi.fn();
    render(<Modal open title="T" onClose={onClose}>x</Modal>);
    fireEvent.keyDown(document, { key: "Escape", isComposing: true });
    expect(onClose).not.toHaveBeenCalled();
  });

  it("renders nothing when closed", () => {
    render(<Modal open={false} title="X" onClose={() => {}}>x</Modal>);
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });
});

describe("Toast", () => {
  function Pusher() {
    const t = useToast();
    return <button onClick={() => t.push("Saved", "success")}>Push</button>;
  }
  it("shows a toast and removes it after a while", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    render(
      <ToastProvider>
        <Pusher />
      </ToastProvider>,
    );
    await userEvent.click(screen.getByRole("button", { name: "Push" }));
    expect(screen.getByText("Saved")).toBeInTheDocument();
    act(() => void vi.advanceTimersByTime(TOAST_MS + 10));
    expect(screen.queryByText("Saved")).not.toBeInTheDocument();
  });
});
