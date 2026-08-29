import { describe, expect, it } from "vitest";

import { isHappeningEvent } from "@/lib/today";

describe("Today event presentation", () => {
  it.each([
    { bucket: "happening_now", urgency: "now", status: "in_progress" },
    { bucket: "happening_now", urgency: "today", status: "scheduled" },
    { bucket: "scheduled_today", urgency: "now", status: "scheduled" },
    { bucket: "scheduled_today", urgency: "today", status: "in_progress" },
  ])("recognizes the backend happening-now signals", (item) => {
    expect(isHappeningEvent(item)).toBe(true);
  });

  it("keeps a future event scheduled", () => {
    expect(
      isHappeningEvent({
        bucket: "scheduled_today",
        urgency: "today",
        status: "scheduled",
      }),
    ).toBe(false);
  });
});
