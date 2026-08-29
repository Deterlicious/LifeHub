export type Priority = "low" | "normal" | "high";
export type RecurrenceFrequency = "daily" | "weekly" | "monthly" | "yearly";

export interface RecurrenceInput {
  frequency: RecurrenceFrequency;
  interval?: number;
  ends_on?: string;
}

export interface RecurrenceSeries {
  id: string;
  sourceKind: "task" | "event" | "bill";
  title: string;
  frequency: RecurrenceFrequency;
  interval: number;
  anchorOn: string;
  endsOn: string | null;
  timezone: string;
  active: boolean;
  createdAt: string;
  updatedAt: string;
}

export type SmartCaptureKind = "task" | "event" | "bill" | "document";

export interface SmartCaptureDraft {
  kind: SmartCaptureKind;
  title: string;
  notes: string;
  priority: Priority;
  dueLocal: string;
  allDay: boolean | null;
  startsLocal: string;
  endsLocal: string;
  startsOn: string;
  endsOn: string;
  location: string;
  amount: number | null;
  currency: string;
  name: string;
  category: DocumentCategory;
  expiresOn: string;
  recurrence: RecurrenceInput | null;
  confidence: number;
}

export interface SmartCaptureResult {
  draft: SmartCaptureDraft;
  ambiguities: string[];
  provider: string;
}

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

export interface Event {
  id: string;
  title: string;
  notes: string | null;
  location: string | null;
  allDay: boolean;
  timezone: string;
  startsAt: string | null;
  endsAt: string | null;
  startsOn: string | null;
  endsOn: string | null;
  createdAt: string;
  updatedAt: string;
}

export interface Bill {
  id: string;
  title: string;
  notes: string | null;
  amount: number;
  currency: string;
  dueAt: string;
  paidAt: string | null;
  createdAt: string;
  updatedAt: string;
}

export type DocumentCategory =
  | "identity"
  | "license"
  | "insurance"
  | "education"
  | "work"
  | "other";

export interface DocumentRecord {
  id: string;
  name: string;
  category: DocumentCategory;
  notes: string | null;
  expiresOn: string;
  status: "valid" | "expiring" | "expired";
  daysUntilExpiry: number;
  createdAt: string;
  updatedAt: string;
}

interface TodayItemBase {
  id: string;
  title: string;
  notes: string | null;
  urgency: string;
  status: string;
  bucket: string;
}

export interface TodayTaskItem extends TodayItemBase {
  kind: "task";
  priority: Priority;
  dueAt: string | null;
  completedAt: string | null;
}

export interface TodayEventItem extends TodayItemBase {
  kind: "event";
  location: string | null;
  allDay: boolean;
  timezone: string;
  startsAt: string | null;
  endsAt: string | null;
  startsOn: string | null;
  endsOn: string | null;
}

export interface TodayBillItem extends TodayItemBase {
  kind: "bill";
  amount: number;
  currency: string;
  dueAt: string;
  paidAt: string | null;
}

export interface TodayDocumentItem extends TodayItemBase {
  kind: "document";
  category: DocumentCategory;
  expiresOn: string;
  daysUntilExpiry: number;
  status: "expired" | "expiring";
  bucket: "expired" | "expires_today" | "expiring_soon";
  urgency: "overdue" | "today" | "upcoming";
}

export type TodayItem =
  | TodayTaskItem
  | TodayEventItem
  | TodayBillItem
  | TodayDocumentItem;

export type EditableTodayItem = TodayTaskItem | TodayEventItem | TodayBillItem;

export interface TodaySummary {
  open: number;
  completed: number;
  upcoming: number;
}

export interface Today {
  date: string;
  timezone: string;
  items: TodayItem[];
  upcoming: TodayItem[];
  upcomingHorizonDays: number;
  summary: TodaySummary;
}

interface AgendaItemBase {
  id: string;
  title: string;
  notes: string | null;
  displayOn: string;
  createdAt: string;
  updatedAt: string;
  status: string;
}

export interface AgendaTaskItem extends AgendaItemBase {
  kind: "task";
  priority: Priority;
  dueAt: string | null;
  completedAt: string | null;
}

export interface AgendaEventItem extends AgendaItemBase {
  kind: "event";
  location: string | null;
  allDay: boolean;
  timezone: string;
  startsAt: string | null;
  endsAt: string | null;
  startsOn: string | null;
  endsOn: string | null;
}

export interface AgendaBillItem extends AgendaItemBase {
  kind: "bill";
  amount: number;
  currency: string;
  dueAt: string;
  paidAt: string | null;
}

export interface AgendaDocumentItem extends AgendaItemBase {
  kind: "document";
  category: DocumentCategory;
  expiresOn: string;
  daysUntilExpiry: number;
  status: DocumentRecord["status"];
}

export type AgendaItem =
  | AgendaTaskItem
  | AgendaEventItem
  | AgendaBillItem
  | AgendaDocumentItem;

export type CorrectableItem =
  | TodayTaskItem
  | TodayEventItem
  | TodayBillItem
  | AgendaTaskItem
  | AgendaEventItem
  | AgendaBillItem;

export interface AgendaSummary {
  total: number;
  tasks: number;
  events: number;
  bills: number;
  documents: number;
}

export interface Agenda {
  from: string;
  to: string;
  timezone: string;
  items: AgendaItem[];
  summary: AgendaSummary;
}

export interface CreateTaskInput {
  title: string;
  notes?: string;
  priority: Priority;
  due_local?: string;
  recurrence?: RecurrenceInput;
}

interface CreateEventBase {
  title: string;
  notes?: string;
  location?: string;
  recurrence?: RecurrenceInput;
}

export type CreateEventInput = CreateEventBase &
  (
    | {
        all_day: false;
        starts_local: string;
        ends_local?: string;
      }
    | {
        all_day: true;
        starts_on: string;
        ends_on?: string;
      }
  );

export interface CreateBillInput {
  title: string;
  notes?: string;
  amount: number;
  currency?: string;
  due_local: string;
  recurrence?: RecurrenceInput;
}

export interface CreateDocumentInput {
  name: string;
  category: DocumentCategory;
  notes?: string;
  expires_on: string;
}

export interface UpdateDocumentInput {
  name: string;
  category: DocumentCategory;
  notes?: string | null;
  expires_on: string;
}

export interface UpdateTaskInput {
  title?: string;
  notes?: string | null;
  priority?: Priority;
  due_local?: string | null;
}

interface UpdateEventMetadataInput {
  title?: string;
  notes?: string | null;
  location?: string | null;
}

type UpdateEventWithoutSchedule = {
  all_day?: never;
  starts_local?: never;
  ends_local?: never;
  starts_on?: never;
  ends_on?: never;
};

export type UpdateEventInput = UpdateEventMetadataInput &
  (
    | UpdateEventWithoutSchedule
    | {
        all_day: false;
        starts_local: string;
        ends_local?: string | null;
        starts_on?: never;
        ends_on?: never;
      }
    | {
        all_day: true;
        starts_on: string;
        ends_on?: string | null;
        starts_local?: never;
        ends_local?: never;
      }
  );

export interface UpdateBillInput {
  title?: string;
  notes?: string | null;
  amount?: number;
  currency?: string;
  due_local?: string;
}

export interface BillListPage {
  items: Bill[];
  nextCursor: string | null;
}

export type ReminderSourceKind = "task" | "event" | "bill" | "document";

export interface BeforeMomentSchedule {
  kind: "before_moment";
  minutesBefore: number;
}

export interface BeforeDateSchedule {
  kind: "before_date";
  daysBefore: number;
  timeLocal: string;
}

export type ReminderSchedule = BeforeMomentSchedule | BeforeDateSchedule;

export interface Reminder {
  id: string;
  sourceKind: ReminderSourceKind;
  sourceId: string;
  schedule: ReminderSchedule;
  status: "scheduled" | "delivered" | "inactive";
  nextFireAt: string | null;
  createdAt: string;
  updatedAt: string;
}

export type ReminderScheduleInput =
  | {
      kind: "before_moment";
      minutes_before: number;
    }
  | {
      kind: "before_date";
      days_before: number;
      time_local: string;
    };

export interface CreateReminderInput {
  source_kind: ReminderSourceKind;
  source_id: string;
  schedule: ReminderScheduleInput;
}

export interface UpdateReminderInput {
  schedule: ReminderScheduleInput;
}

export interface NotificationItem {
  id: string;
  sourceKind: ReminderSourceKind;
  sourceId: string;
  title: string;
  body: string;
  createdAt: string;
  readAt: string | null;
}

export interface NotificationsPage {
  items: NotificationItem[];
  nextCursor: string | null;
  unreadCount: number;
}

export interface MarkNotificationReadResult {
  item: NotificationItem;
  unreadCount: number;
}

export interface MarkAllNotificationsReadResult {
  markedRead: number;
  unreadCount: number;
}

export interface DevSession {
  accessToken: string;
}
