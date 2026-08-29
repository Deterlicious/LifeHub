import { describe, expect, it } from "vitest";

import {
  createLatestRequestGate,
  mergeNewestPageById,
  mergeUniqueById,
} from "@/lib/request-state";

describe("latest request state", () => {
  it("rejects a response after a newer request begins", () => {
    const gate = createLatestRequestGate();
    const first = gate.begin();
    const second = gate.begin();

    expect(gate.isCurrent(first)).toBe(false);
    expect(gate.isCurrent(second)).toBe(true);
  });

  it("invalidates a notification poll that begins while mark-read is in flight", () => {
    const gate = createLatestRequestGate();
    const beforeActionPoll = gate.begin();
    gate.begin(); // mutation begins and invalidates work already in flight
    const duringActionPoll = gate.begin();
    const mutationCommit = gate.begin(); // successful POST is authoritative

    expect(gate.isCurrent(beforeActionPoll)).toBe(false);
    expect(gate.isCurrent(duringActionPoll)).toBe(false);
    expect(gate.isCurrent(mutationCommit)).toBe(true);
  });

  it("updates duplicate history rows without appending them twice", () => {
    expect(
      mergeUniqueById(
        [{ id: "bill-1", title: "Lama" }],
        [
          { id: "bill-1", title: "Diperbarui" },
          { id: "bill-2", title: "Baru" },
        ],
      ),
    ).toEqual([
      { id: "bill-1", title: "Diperbarui" },
      { id: "bill-2", title: "Baru" },
    ]);
  });

  it("refreshes the newest notification page without dropping loaded older pages", () => {
    expect(
      mergeNewestPageById(
        [
          { id: "notification-2", read: false },
          { id: "notification-1", read: false },
          { id: "notification-old", read: false },
        ],
        [
          { id: "notification-3", read: false },
          { id: "notification-2", read: true },
          { id: "notification-1", read: true },
        ],
      ),
    ).toEqual([
      { id: "notification-3", read: false },
      { id: "notification-2", read: true },
      { id: "notification-1", read: true },
      { id: "notification-old", read: false },
    ]);
  });
});
