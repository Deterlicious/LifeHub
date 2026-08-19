export type Priority = "low" | "normal" | "high";

export interface Profile {
  timezone: string;
  locale: string;
  currency: string;
}

export interface Task {
  id: string;
  title: string;
  notes: string | null;
  priority: Priority;
  dueAt: string | null;
  completedAt: string | null;
}

export interface TodayItem extends Task {
  kind: string;
  urgency: string;
  status: string;
}

export interface TodaySummary {
  open: number;
  completed: number;
}

export interface Today {
  date: string;
  timezone: string;
  items: TodayItem[];
  summary: TodaySummary;
}

export interface CreateTaskInput {
  title: string;
  notes?: string;
  priority: Priority;
  due_local?: string;
}

export interface DevSession {
  accessToken: string;
}
