import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import type { TaskObjectiveCriterion } from "@/lib/types/objective";
import { ObjectivePanel } from "./objective-panel";

function criterion(overrides: Partial<TaskObjectiveCriterion> = {}): TaskObjectiveCriterion {
  return {
    id: "c1",
    run_id: "r1",
    task_id: "t1",
    ordinal: 0,
    source: "derived",
    source_ref: "",
    text: "Adds a button",
    status: "met",
    rationale: "",
    evidence: [],
    created_at: "2026-08-01T10:00:00Z",
    ...overrides,
  };
}

afterEach(cleanup);

describe("ObjectivePanel", () => {
  it("shows the empty state with no verdict and no criteria", () => {
    render(
      <ObjectivePanel
        verdict=""
        summary=""
        criteria={[]}
        changedFileKeys={new Set()}
        onNavigateToFile={vi.fn()}
      />,
    );
    expect(screen.getByTestId("objective-panel-empty")).toBeTruthy();
  });

  it("renders the banner, every criterion, and the unmet count for a partial verdict", () => {
    render(
      <ObjectivePanel
        verdict="partial"
        summary="Half done"
        criteria={[
          criterion({ id: "a", text: "First", status: "met" }),
          criterion({ id: "b", text: "Second", status: "unmet" }),
          criterion({ id: "c", text: "Third", status: "unknown" }),
        ]}
        changedFileKeys={new Set()}
        onNavigateToFile={vi.fn()}
      />,
    );
    expect(screen.getByText("Objective partially met")).toBeTruthy();
    expect(screen.getByText("Half done")).toBeTruthy();
    expect(screen.getAllByTestId("objective-criterion-row")).toHaveLength(3);
    expect(screen.getByText("2 of 3 not met")).toBeTruthy();
  });

  it("links navigable evidence and renders unchanged-file evidence as plain text", () => {
    const onNavigateToFile = vi.fn();
    render(
      <ObjectivePanel
        verdict="unmet"
        summary=""
        criteria={[
          criterion({
            id: "a",
            text: "Needs work",
            status: "unmet",
            rationale: "because",
            evidence: [
              { file: "src/changed.ts", line: 3 },
              { file: "src/untouched.ts", line: 9 },
            ],
          }),
        ]}
        changedFileKeys={new Set(["src/changed.ts"])}
        onNavigateToFile={onNavigateToFile}
      />,
    );
    fireEvent.click(screen.getByText("Needs work"));
    fireEvent.click(screen.getByTestId("objective-evidence-link"));
    expect(onNavigateToFile).toHaveBeenCalledWith("src/changed.ts");
    expect(screen.getByTestId("objective-evidence-text").textContent).toContain("src/untouched.ts");
  });

  it("sends a criterion to the agent", () => {
    const onSendToAgent = vi.fn();
    const target = criterion({ id: "a", text: "Ship it", status: "unmet" });
    render(
      <ObjectivePanel
        verdict="unmet"
        summary=""
        criteria={[target]}
        changedFileKeys={new Set()}
        onNavigateToFile={vi.fn()}
        onSendToAgent={onSendToAgent}
      />,
    );
    fireEvent.click(screen.getByTestId("objective-send-criterion"));
    expect(onSendToAgent).toHaveBeenCalledWith(target);
  });
});
