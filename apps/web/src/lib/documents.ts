import type { DocumentCategory } from "@/lib/api/types";

export const DOCUMENT_CATEGORY_OPTIONS: ReadonlyArray<{
  value: DocumentCategory;
  label: string;
}> = [
  { value: "identity", label: "Identitas" },
  { value: "license", label: "Lisensi" },
  { value: "insurance", label: "Asuransi" },
  { value: "education", label: "Pendidikan" },
  { value: "work", label: "Pekerjaan" },
  { value: "other", label: "Lainnya" },
];

export function documentCategoryLabel(category: DocumentCategory): string {
  return DOCUMENT_CATEGORY_OPTIONS.find((option) => option.value === category)?.label ?? "Lainnya";
}

export function documentStatusLabel(status: string): string {
  if (status === "expired") return "Kedaluwarsa";
  if (status === "expiring") return "Segera kedaluwarsa";
  return "Masih berlaku";
}
