"use client";

import { Suspense, useEffect, useState } from "react";
import { useParams, useRouter } from "next/navigation";
import { Alert, Card, Shell, Spinner } from "@/components/ui";
import { api, errorText, useJourney } from "@/lib/journey";

/** Landing page of the WhatsApp/SMS resume link: restores the lead and sends the owner to the KYC step.
 *  The token is only known at request time, so the reader sits inside Suspense (Cache Components). */
export default function ResumePage() {
  return (
    <Suspense fallback={<Spinner />}>
      <Resume />
    </Suspense>
  );
}

function Resume() {
  const { token } = useParams<{ token: string }>();
  const { update, t } = useJourney();
  const router = useRouter();
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    api.kyc.resume(token).then((r) => {
      update({ leadId: r.leadId, consented: true });
      router.replace("/kyc");
    }).catch((e) => setError(errorText(e, t("error"))));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [token]);

  return (
    <Shell step={3} title={t("resumeWelcome")}>
      <Card>{error ? <Alert tone="error">{error}</Alert> : <Spinner />}</Card>
    </Shell>
  );
}
