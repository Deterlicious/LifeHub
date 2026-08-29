import type { TodayEventItem } from "@/lib/api/types";

export function isHappeningEvent(
  item: Pick<TodayEventItem, "bucket" | "status" | "urgency">,
): boolean {
  return item.bucket === "happening_now"
    || item.urgency === "now"
    || item.status === "in_progress";
}
