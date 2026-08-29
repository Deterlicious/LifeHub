import { describe, expect, it } from "vitest";

import { documentCategoryLabel, documentStatusLabel } from "@/lib/documents";

describe("LifeHub document labels", () => {
  it("uses Indonesian labels for persisted category enums", () => {
    expect(documentCategoryLabel("identity")).toBe("Identitas");
    expect(documentCategoryLabel("insurance")).toBe("Asuransi");
    expect(documentCategoryLabel("other")).toBe("Lainnya");
  });

  it("distinguishes expired and approaching documents without relying on color", () => {
    expect(documentStatusLabel("expired")).toBe("Kedaluwarsa");
    expect(documentStatusLabel("expiring")).toBe("Segera kedaluwarsa");
    expect(documentStatusLabel("valid")).toBe("Masih berlaku");
  });
});
