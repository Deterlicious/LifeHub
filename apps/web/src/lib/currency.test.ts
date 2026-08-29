import { describe, expect, it } from "vitest";

import { formatMoney, parseIntegerAmount } from "@/lib/currency";

describe("LifeHub money helpers", () => {
  it("formats integer rupiah without fractional units", () => {
    expect(formatMoney(350000, "IDR")).toBe("Rp350.000");
  });

  it("accepts only positive safe integers from the bill form", () => {
    expect(parseIntegerAmount("350000")).toBe(350000);
    expect(parseIntegerAmount("350.5")).toBeNull();
    expect(parseIntegerAmount("0")).toBeNull();
    expect(parseIntegerAmount("9007199254740992")).toBeNull();
  });
});
