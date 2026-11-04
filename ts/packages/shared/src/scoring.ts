// Scoring service and advisor handoff shapes. Contract: contracts/scoring.schema.json (Go: go/pkg/scoring, go/cmd/scoring).

export interface ScoreReason {
  field: string;
  value: string;
  effect: "raises" | "lowers";
  /** log-odds */
  contribution: number;
  text: string;
}

export interface LeadScore {
  probability: number;
  band: "high" | "medium" | "low";
  reasons: ScoreReason[];
  model: string;
}

export interface ScoredLead {
  leadId: string;
  source: "live" | "case";
  businessName: string;
  city: string;
  segment: string;
  leadSource: string;
  probability: number;
  band: "high" | "medium" | "low";
  reasons: ScoreReason[];
  updatedAt: string;
}

export interface PartnerTask {
  id: string;
  leadId: string;
  reason: string;
  summary: string;
  transcript: { role: string; text: string }[];
  status: "open" | "claimed" | "closed";
  partner?: string;
  probability?: number;
  band?: string;
  reasons?: ScoreReason[];
  createdAt: string;
}

export type HandoffReason = "owner_asked_for_person" | "custom_request" | "not_ready";
