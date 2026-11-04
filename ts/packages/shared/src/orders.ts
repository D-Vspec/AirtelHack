// Tracker and notify HTTP shapes. Contract: contracts/orders.schema.json (Go: go/pkg/orders,
// go/cmd/tracker, go/cmd/notify).

export const ORDER_HAPPY_PATH = ["quote_accepted", "kyc_pending", "kyc_passed", "install_scheduled", "installing", "activated"] as const;
export type OrderStatus = (typeof ORDER_HAPPY_PATH)[number] | "delayed" | "failed";

export interface OrderLine {
  productId: string;
  productName: string;
  quantity: number;
  installDays: number;
}

export interface Order {
  id: string;
  leadId: string;
  quoteId: string;
  status: OrderStatus;
  /** e.g. fibre_not_feasible, kyc_rejected, fallback_5g_accepted */
  statusReason?: string;
  productIds: string[];
  lines: OrderLine[];
  monthlyTotalInr: number;
  oneTimeTotalInr: number;
  /** slowest product's typical_install_days */
  expectedInstallDays: number;
  /** YYYY-MM-DD: KYC pass date (or order date until then) + expectedInstallDays */
  expectedActivationDate?: string;
  fallbackOffered: boolean;
  createdAt: string;
  updatedAt: string;
}

export interface TimelineEntry {
  status: OrderStatus;
  reason?: string;
  note?: string;
  at: string;
}

export interface TimelineStep {
  status: (typeof ORDER_HAPPY_PATH)[number];
  state: "done" | "current" | "upcoming" | "blocked";
  at?: string;
}

export interface Timeline {
  order: Order;
  entries: TimelineEntry[];
  steps: TimelineStep[];
}

export interface Transition {
  to: OrderStatus;
  reason?: string;
  note?: string;
}

export interface SentMessage {
  id: string;
  eventId: string;
  leadId: string;
  channel: "whatsapp" | "sms" | "email";
  templateKey: string;
  language: string;
  recipient: string;
  body: string;
  createdAt: string;
}

export interface ResumeState {
  leadId: string;
  status: string;
  documents: string[];
  missingDocuments: string[];
  actionNeeded: import("./kyc").KycFinding[];
  expiresAt: string;
}
