// SPDX-License-Identifier: MIT
import { act, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useState } from "react";
import { Alert, Badge, Button, Chip, ConfirmDialog, Field, Input, LinkButton, Modal, Select, Table, ToastProvider, TOAST_MS, useToast } from ".";

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
    expect(confirm).toHaveFocus();
    await userEvent.tab();
    expect(cancel).toHaveFocus();
    await userEvent.tab();
    expect(confirm).toHaveFocus();
    await userEvent.tab({ shift: true });
    expect(cancel).toHaveFocus();
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
