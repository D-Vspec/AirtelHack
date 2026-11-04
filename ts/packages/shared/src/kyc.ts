// KYC service HTTP shapes. Contract: contracts/kyc.schema.json (Go: go/pkg/kyc, go/cmd/kyc).
import type { KycDocumentType, KycFindingCode, KycRoute, KycSeverity } from "./kyc-findings";

export const KYC_PURPOSES = ["kyc_verification", "service_provisioning", "communication"] as const;
export type KycPurpose = (typeof KYC_PURPOSES)[number];

export const KYC_STAGES = [
  "completeness", "image_quality", "ocr", "identifiers", "registry_status",
  "name_match", "address_match", "document_age", "signatory_id",
] as const;
export type KycStage = (typeof KYC_STAGES)[number];

export interface ConsentRequest {
  leadId: string;
  purposes: KycPurpose[];
  version: string;
  language?: string;
}

export interface Consent extends Required<Omit<ConsentRequest, "language">> {
  id: string;
  language?: string;
  grantedAt: string;
  withdrawnAt: string | null;
}

export interface Withdrawal {
  consentId: string;
  leadId: string;
  withdrawnAt: string;
  purgedDocuments: number;
}

export interface KycFinding {
  code: KycFindingCode;
  severity: KycSeverity;
  route: KycRoute;
  stage: KycStage;
  document?: KycDocumentType;
  /** kyc.finding.<code>; text per language in KYC_MESSAGES */
  messageKey: string;
  params?: Record<string, string>;
  /** English explanation for reviewers (identifiers masked) */
  detail?: string;
}

export interface Prefill {
  matchType: "exact" | "corrected" | "none";
  checksumValid: boolean;
  suggestionCount: number;
  gstin?: string;
  legalName?: string;
  tradeName?: string;
  constitution?: string;
  address?: string;
  state?: string;
  status?: string;
}

export interface UploadResult {
  leadId: string;
  type: KycDocumentType;
  bytes: number;
  sharpness: number;
  blurry: boolean;
  skewDeg: number;
  /** instant image-quality findings (IMAGE_BLURRY, IMAGE_SKEWED) */
  findings: KycFinding[];
}

export type KycStatus = "collecting" | KycRoute;

export interface KycApplication {
  leadId: string;
  status: KycStatus;
  documents: KycDocumentType[];
  route?: KycRoute;
  findings?: KycFinding[];
  verified?: {
    gstin?: string;
    gstinMatchType?: "exact" | "corrected" | "none";
    gstinRecord?: { gstin: string; legalName: string; tradeName: string; constitution: string; address: string; state: string; status: string };
    udyamNumber?: string;
    udyamMatchType?: "exact" | "corrected" | "none";
    udyamRecord?: { udyamNumber: string; enterpriseName: string; majorActivity: string; city: string; state: string };
  };
  reviewId?: string;
  checkedAt?: string;
}

export type ReviewDecisionKind = "approve" | "reject" | "request_reupload";

export interface KycReview {
  id: string;
  leadId: string;
  status: "open" | "decided";
  codes: KycFindingCode[];
  findings?: KycFinding[];
  decision?: ReviewDecisionKind;
  reviewer?: string;
  note?: string;
  createdAt: string;
  decidedAt?: string;
}

export interface ReviewDecision {
  decision: ReviewDecisionKind;
  reviewer: string;
  note?: string;
}
