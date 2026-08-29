import { describe, expect, it } from "vitest";

import { appViewFromHash, isKnownAppHash } from "@/lib/navigation";

describe("LifeHub hash navigation", () => {
  it("keeps Today as the default and maps only the real Agenda surface", () => {
    expect(appViewFromHash("")).toBe("today");
    expect(appViewFromHash("#today")).toBe("today");
    expect(appViewFromHash("#quick-add")).toBe("today");
    expect(appViewFromHash("#agenda")).toBe("agenda");
    expect(appViewFromHash("#calendar-palsu")).toBe("today");
  });

  it("recognizes only hashes backed by a working UI target", () => {
    expect(isKnownAppHash("#today")).toBe(true);
    expect(isKnownAppHash("#agenda")).toBe(true);
    expect(isKnownAppHash("#quick-add")).toBe(true);
    expect(isKnownAppHash("#documents")).toBe(false);
  });
});
