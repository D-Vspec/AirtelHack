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

// ---- Customer accounts (contracts/kyc.schema.json OtpSent, Account, AccountView, ReuseStatus) ----

export interface OtpSent { sent: boolean; mobileMasked: string; expiresInSeconds: number; /** prototype only */ demoCode?: string }
export interface Account { id: string; mobileMasked: string; createdAt: string }
export interface SignIn { token: string; account: Account }
export interface VerifiedBusiness {
  fromLeadId: string; verifiedAt: string; gstinMasked: string; udyamMasked: string;
  legalName?: string; tradeName?: string; address?: string;
}
export interface AccountLead { leadId: string; linkedAt: string; kycStatus: string; reusedKyc: boolean }
export interface AccountView { account: Account; leads: AccountLead[]; verified: VerifiedBusiness | null }

export const REUSE_REASONS = ["NO_VERIFICATION", "OWNER_CHANGED", "VERIFICATION_EXPIRED", "SIGNATORY_ID_EXPIRED", "GSTIN_NOT_ACTIVE",
  "REGISTRY_DETAILS_CHANGED", "UDYAM_NOT_FOUND", "REGISTRY_UNAVAILABLE"] as const;
export type ReuseReason = (typeof REUSE_REASONS)[number];
export interface ReuseStatus { reusable: boolean; reasons: ReuseReason[]; verified: VerifiedBusiness | null }

/** Why the earlier verification can't be reused, in plain English (the KYC screen asks for documents). */
export const REUSE_REASON_TEXT: Record<ReuseReason, string> = {
  NO_VERIFICATION: "No verified documents on your account yet.",
  OWNER_CHANGED: "You told us your business details changed.",
  VERIFICATION_EXPIRED: "Your last verification is more than a year old.",
  SIGNATORY_ID_EXPIRED: "The signatory ID we verified has expired.",
  GSTIN_NOT_ACTIVE: "The GST registry no longer shows your GSTIN as active.",
  REGISTRY_DETAILS_CHANGED: "Your business name or address changed in the GST registry.",
  UDYAM_NOT_FOUND: "Your Udyam number is no longer in the registry.",
  REGISTRY_UNAVAILABLE: "We couldn't reach the registry to re-check your details.",
};
