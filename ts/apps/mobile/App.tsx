import { StatusBar } from "expo-status-bar";
import * as ImagePicker from "expo-image-picker";
import { useEffect, useRef, useState, type ComponentRef, type ReactNode } from "react";
import { ActivityIndicator, KeyboardAvoidingView, Platform, Pressable, ScrollView, StyleSheet, Text, TextInput, View } from "react-native";
import {
  createApiClient, DEMO_OWNERS, inr, KYC_MESSAGES, LANGUAGES, newLeadId, ORDER_STATUS_LABELS, t as translate,
  type BusinessProfile, type KycApplication, type KycDocumentType, type KycFinding, type LangCode, type Quote, type SentMessage,
  type StringKey, type Timeline,
} from "@udyam/shared";

// On a physical phone use the computer's LAN IP, e.g. EXPO_PUBLIC_API_URL=http://192.168.1.20:8080/api
const api = createApiClient(process.env.EXPO_PUBLIC_API_URL ?? "http://localhost:8080/api");

type Screen = "home" | "chat" | "quote" | "consent" | "kyc" | "track";
const STEPS: Screen[] = ["chat", "quote", "consent", "kyc", "track"];
const STEP_KEYS: Record<Screen, StringKey> = { home: "appName", chat: "stepChat", quote: "stepQuote", consent: "stepConsent", kyc: "stepKyc", track: "stepTrack" };

interface Msg { role: "owner" | "advisor"; text: string }
interface Journey { lang: LangCode; leadId: string; sessionId?: string; messages: Msg[]; profile: BusinessProfile; quoteId?: string; specimen: string; draft: string }

const errText = (e: unknown, fallback: string) => (e as { detail?: string })?.detail || fallback;

export default function App() {
  const [screen, setScreen] = useState<Screen>("home");
  const [j, setJ] = useState<Journey>({ lang: "en", leadId: "", messages: [], profile: {}, specimen: "K03", draft: "" });
  const t = (k: StringKey, p?: Record<string, string | number>) => translate(j.lang, k, p);
  const update = (patch: Partial<Journey>) => setJ((cur) => ({ ...cur, ...patch }));
  const props = { j, update, t, go: setScreen };

  return (
    <KeyboardAvoidingView style={{ flex: 1, backgroundColor: "#f5f7fb" }} behavior={Platform.OS === "ios" ? "padding" : undefined}>
      <View style={s.header}>
        <Text style={s.brand} onPress={() => setScreen("home")}>{t("appName")}</Text>
        {screen !== "home" && <Text style={s.stepLabel}>{STEPS.indexOf(screen) + 1}/5 · {t(STEP_KEYS[screen])}</Text>}
      </View>
      {screen !== "home" && (
        <View style={s.progress}>{STEPS.map((x, i) => <View key={x} style={[s.bar, i <= STEPS.indexOf(screen) && s.barOn]} />)}</View>
      )}
      {screen === "home" && <Home {...props} />}
      {screen === "chat" && <Chat {...props} />}
      {screen === "quote" && <QuoteScreen {...props} />}
      {screen === "consent" && <Consent {...props} />}
      {screen === "kyc" && <Kyc {...props} />}
      {screen === "track" && <Track {...props} />}
      <StatusBar style="dark" />
    </KeyboardAvoidingView>
  );
}

interface P { j: Journey; update: (p: Partial<Journey>) => void; t: (k: StringKey, p?: Record<string, string | number>) => string; go: (s: Screen) => void }

function Btn({ title, onPress, disabled, variant = "primary", testID, busy }: { title: string; onPress: () => void; disabled?: boolean; variant?: "primary" | "secondary"; testID?: string; busy?: boolean }) {
  return (
    <Pressable testID={testID} accessibilityRole="button" disabled={disabled || busy} onPress={onPress}
      style={[s.btn, variant === "secondary" && s.btnSecondary, (disabled || busy) && s.btnDisabled]}>
      {busy ? <ActivityIndicator color="#fff" /> : <Text style={[s.btnText, variant === "secondary" && s.btnTextSecondary]}>{title}</Text>}
    </Pressable>
  );
}

function Page({ children, footer }: { children: ReactNode; footer?: ReactNode }) {
  return (
    <View style={{ flex: 1 }}>
      <ScrollView contentContainerStyle={s.page} keyboardShouldPersistTaps="handled">{children}</ScrollView>
      {footer && <View style={s.footer}>{footer}</View>}
    </View>
  );
}

const Card = ({ children }: { children: ReactNode }) => <View style={s.card}>{children}</View>;
const ErrorBox = ({ text }: { text: string | null }) => (text ? <Text style={s.error}>{text}</Text> : null);

function Home({ j, update, t, go }: P) {
  const start = (lang: LangCode, prefix = "mobile", specimen = "K03", draft = "") => {
    update({ lang, leadId: newLeadId(prefix), sessionId: undefined, messages: [], profile: {}, quoteId: undefined, specimen, draft });
    go("chat");
  };
  return (
    <Page>
      <Text style={s.h1}>{t("appName")}</Text>
      <Text style={s.muted}>{t("tagline")}</Text>
      <Card>
        <Text style={s.h2}>{t("chooseLanguage")}</Text>
        <View style={s.grid}>
          {LANGUAGES.map((l) => (
            <Pressable key={l.code} onPress={() => update({ lang: l.code })} style={[s.lang, j.lang === l.code && s.langOn]}>
              <Text style={[s.langText, j.lang === l.code && { color: "#fff", fontWeight: "700" }]}>{l.native}</Text>
            </Pressable>
          ))}
        </View>
        <Btn title={`${t("start")} →`} onPress={() => start(j.lang)} />
      </Card>
      <Card>
        <Text style={s.h2}>{t("demoOwners")}</Text>
        {DEMO_OWNERS.map((o) => (
          <Pressable key={o.id} testID={`demo-${o.id}`} onPress={() => start(o.language, o.leadPrefix, o.specimen, o.firstMessage)} style={s.demo}>
            <Text style={{ fontWeight: "600" }}>{o.title}</Text>
            <Text style={s.small}>#{o.scenario} · KYC {o.specimen} · {o.expect}</Text>
          </Pressable>
        ))}
      </Card>
    </Page>
  );
}

function Chat({ j, update, t, go }: P) {
  const [input, setInput] = useState(j.draft);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [quote, setQuote] = useState<Quote | null>(null);
  const scroll = useRef<ComponentRef<typeof ScrollView>>(null);

  async function send(text: string) {
    const message = text.trim();
    if (!message || busy) return;
    setInput("");
    setError(null);
    setBusy(true);
    const messages: Msg[] = [...j.messages, { role: "owner", text: message }];
    update({ messages, draft: "" });
    try {
      const r = await api.advisor.chat({ message, sessionId: j.sessionId, leadId: j.leadId });
      update({ messages: [...messages, { role: "advisor", text: r.reply }], sessionId: r.sessionId, profile: r.profile, quoteId: r.quote?.id ?? j.quoteId });
      if (r.quote) setQuote(r.quote);
    } catch (e) {
      setError(errText(e, t("error")));
      setInput(message);
    } finally {
      setBusy(false);
    }
  }

  return (
    <View style={{ flex: 1 }}>
      <ScrollView ref={scroll} onContentSizeChange={() => scroll.current?.scrollToEnd()} contentContainerStyle={s.page}>
        {j.messages.map((m, i) => (
          <Text key={i} style={[s.bubble, m.role === "owner" ? s.mine : s.theirs]}>{m.text}</Text>
        ))}
        {busy && <ActivityIndicator style={{ alignSelf: "flex-start" }} color="#1f5bd8" />}
        <ErrorBox text={error} />
        {quote && (
          <Card>
            <Text style={s.price}>{inr(quote.monthlyTotalInr)} <Text style={s.muted}>{t("perMonth")}</Text></Text>
            <Text style={s.small}>{inr(quote.oneTimeTotalInr)} {t("oneTime")} · {t("installIn", { days: quote.expectedInstallDays })}</Text>
          </Card>
        )}
      </ScrollView>
      <View style={s.footer}>
        {quote ? (
          <Btn testID="see-quote" title={`${t("seeQuote")} →`} onPress={() => go("quote")} />
        ) : null}
        <View style={{ flexDirection: "row", gap: 8, marginTop: quote ? 8 : 0 }}>
          <TextInput testID="msg" value={input} onChangeText={setInput} placeholder={t("chatPlaceholder")} multiline style={s.input} />
          <Btn testID="send" title={t("send")} disabled={!input.trim()} busy={busy} onPress={() => send(input)} />
        </View>
      </View>
    </View>
  );
}

function QuoteScreen({ j, t, go }: P) {
  const [quote, setQuote] = useState<Quote | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  useEffect(() => {
    if (j.quoteId) api.quotes.get(j.quoteId).then(setQuote).catch((e) => setError(errText(e, t("error"))));
  }, [j.quoteId]); // eslint-disable-line react-hooks/exhaustive-deps

  async function accept() {
    if (!quote) return;
    setBusy(true);
    try {
      await api.quotes.accept(quote.id);
      go("consent");
    } catch (e) {
      setError(errText(e, t("error")));
      setBusy(false);
    }
  }
  return (
    <Page footer={quote && <Btn testID="accept" title={t("accept")} busy={busy} onPress={accept} />}>
      {!quote && !error && <ActivityIndicator color="#1f5bd8" />}
      <ErrorBox text={error} />
      {quote && (
        <Card>
          {quote.lines.map((l) => (
            <View key={l.productId} style={s.row}>
              <Text style={{ flex: 1 }}>{l.productName}{l.quantity > 1 ? ` × ${l.quantity}` : ""}</Text>
              <Text style={{ fontWeight: "600" }}>{inr(l.monthlyInr)}</Text>
            </View>
          ))}
          <View style={s.divider} />
          <Text style={s.price}>{inr(quote.monthlyTotalInr)} <Text style={s.muted}>{t("perMonth")}</Text></Text>
          <Text style={s.small}>{inr(quote.oneTimeTotalInr)} {t("oneTime")} · {t("totalContract", { months: quote.contractMonths })}: {inr(quote.totalContractValueInr)}</Text>
          <Text style={s.small}>{t("installIn", { days: quote.expectedInstallDays })}</Text>
        </Card>
      )}
    </Page>
  );
}

function Consent({ j, t, go }: P) {
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  async function agree() {
    setBusy(true);
    try {
      await api.kyc.consent({ leadId: j.leadId, purposes: ["kyc_verification", "service_provisioning", "communication"], version: "2026-07-v1", language: j.lang });
      go("kyc");
    } catch (e) {
      setError(errText(e, t("error")));
      setBusy(false);
    }
  }
  return (
    <Page footer={<Btn testID="agree" title={t("agree")} busy={busy} onPress={agree} />}>
      <Card>
        <Text style={s.h2}>{t("consentTitle")}</Text>
        <Text>{t("consentBody")}</Text>
        {(["consentKyc", "consentProvision", "consentComms"] as StringKey[]).map((k) => <Text key={k} style={s.check}>☑ {t(k)}</Text>)}
      </Card>
      <ErrorBox text={error} />
    </Page>
  );
}

const DOCS: { type: KycDocumentType; label: StringKey }[] = [
  { type: "gst_certificate", label: "docGst" }, { type: "udyam_certificate", label: "docUdyam" },
  { type: "address_proof", label: "docAddress" }, { type: "signatory_id", label: "docSignatory" },
];

function FindingRow({ f, lang }: { f: KycFinding; lang: LangCode }) {
  const m = KYC_MESSAGES[f.code]?.[lang] ?? KYC_MESSAGES[f.code]?.en;
  return (
    <View style={[s.finding, f.route === "reject" && { backgroundColor: "#fee2e2" }]}>
      <Text style={{ fontWeight: "600" }}>{m?.message ?? f.code}</Text>
      {m?.action && <Text style={s.small}>→ {m.action}</Text>}
    </View>
  );
}

function Kyc({ j, t, go }: P) {
  const [have, setHave] = useState<Partial<Record<KycDocumentType, KycFinding[]>>>({});
  const [busy, setBusy] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [result, setResult] = useState<KycApplication | null>(null);

  // Camera capture: rear camera on a phone; the browser build falls back to a file picker.
  async function capture(type: KycDocumentType) {
    setError(null);
    const perm = Platform.OS === "web" ? { granted: true } : await ImagePicker.requestCameraPermissionsAsync();
    const opts: ImagePicker.ImagePickerOptions = { mediaTypes: ["images"], quality: 0.8, allowsEditing: false };
    const pick = perm.granted ? await ImagePicker.launchCameraAsync(opts) : await ImagePicker.launchImageLibraryAsync(opts);
    if (pick.canceled || !pick.assets[0]) return;
    setBusy(type);
    try {
      const blob = await (await fetch(pick.assets[0].uri)).blob();
      const r = await api.kyc.upload(j.leadId, type, blob);
      setHave((h) => ({ ...h, [type]: r.findings }));
    } catch (e) {
      setError(errText(e, t("error")));
    } finally {
      setBusy(null);
    }
  }

  async function sample() {
    setBusy("sample");
    try {
      const r = await api.kyc.loadSpecimen(j.leadId, j.specimen);
      setHave(Object.fromEntries(r.documents.map((d) => [d, []])));
    } catch (e) {
      setError(errText(e, t("error")));
    } finally {
      setBusy(null);
    }
  }

  async function check() {
    setBusy("check");
    try {
      setResult(await api.kyc.check(j.leadId));
    } catch (e) {
      setError(errText(e, t("error")));
    } finally {
      setBusy(null);
    }
  }

  const status = result?.status;
  const statusText = status && ({ pass: t("kycPass"), manual_review: t("kycReview"), reupload: t("kycReupload"), reject: t("kycReject") } as Record<string, string>)[status];
  const canTrack = status === "pass" || status === "manual_review";
  return (
    <Page footer={
      <View style={{ gap: 8 }}>
        <Btn testID="check-docs" title={t("checkDocs")} variant={canTrack ? "secondary" : "primary"} disabled={!Object.keys(have).length} busy={busy === "check"} onPress={check} />
        {canTrack && <Btn testID="go-track" title={`${t("stepTrack")} →`} onPress={() => go("track")} />}
      </View>
    }>
      {DOCS.map(({ type, label }) => (
        <Card key={type}>
          <View style={s.row}>
            <Text style={s.h2}>{t(label)}</Text>
            {(have[type] || status === "pass") && <Text style={{ color: "#15803d" }}>✓ {t("uploaded")}</Text>}
          </View>
          <Pressable onPress={() => capture(type)} disabled={!!busy} style={s.capture}>
            {busy === type ? <ActivityIndicator color="#1f5bd8" /> : <Text style={{ color: "#1748b0" }}>📷 {t("takePhoto")}</Text>}
          </Pressable>
          {[...(have[type] ?? []), ...(result?.findings ?? []).filter((f) => f.document === type)].map((f, i) => <FindingRow key={i} f={f} lang={j.lang} />)}
        </Card>
      ))}
      <Btn testID="use-sample" title={`${t("useSample")} (${j.specimen})`} variant="secondary" busy={busy === "sample"} onPress={sample} />
      <ErrorBox text={error} />
      {statusText && <View testID="kyc-result" style={[s.finding, status === "pass" && { backgroundColor: "#dcfce7" }]}><Text style={{ fontWeight: "700" }}>{statusText}</Text><Text style={{ display: "none" }}>{status}</Text></View>}
    </Page>
  );
}

function Track({ j, t }: P) {
  const [tl, setTl] = useState<Timeline | null>(null);
  const [msgs, setMsgs] = useState<SentMessage[]>([]);
  useEffect(() => {
    let stop = false;
    const poll = async () => {
      try {
        const { orders } = await api.tracker.orders({ leadId: j.leadId });
        if (orders[0]) setTl(await api.tracker.timeline(orders[0].id));
        setMsgs((await api.notify.messages(j.leadId)).messages);
      } catch { /* retry next tick */ }
      if (!stop) setTimeout(poll, 3000);
    };
    void poll();
    return () => { stop = true; };
  }, [j.leadId]);

  const labels = ORDER_STATUS_LABELS[j.lang];
  const o = tl?.order;
  return (
    <Page>
      {!o && <Text>{t("noOrder")}</Text>}
      {o && (
        <Card>
          <Text testID="order-status" style={s.price}>{labels[o.status] ?? o.status}</Text>
          <Text testID={`order-${o.status}`} style={{ display: "none" }}>{o.status}</Text>
          {o.expectedActivationDate && o.status !== "activated" && <Text>{t("expectedBy", { date: o.expectedActivationDate })}</Text>}
          {tl!.steps.map((st) => (
            <View key={st.status} style={s.row}>
              <Text style={[s.dot, st.state === "done" && s.dotDone, st.state === "current" && s.dotNow, st.state === "blocked" && s.dotBlocked]}>{st.state === "done" ? "✓" : st.state === "blocked" ? "!" : "•"}</Text>
              <Text style={{ flex: 1, color: st.state === "upcoming" ? "#6b7280" : "#111827" }}>{labels[st.status]}</Text>
            </View>
          ))}
        </Card>
      )}
      {msgs.length > 0 && (
        <Card>
          <Text style={s.h2}>{t("messagesSent")}</Text>
          {msgs.map((m) => (
            <View key={m.id} style={s.wa}><Text style={s.waChannel}>{m.channel.toUpperCase()}</Text><Text>{m.body}</Text></View>
          ))}
        </Card>
      )}
    </Page>
  );
}

const s = StyleSheet.create({
  header: { paddingTop: Platform.OS === "web" ? 12 : 48, paddingHorizontal: 16, paddingBottom: 8, backgroundColor: "#fff", flexDirection: "row", justifyContent: "space-between", alignItems: "center" },
  brand: { fontSize: 18, fontWeight: "700", color: "#0f3d91" },
  stepLabel: { color: "#0f3d91", fontWeight: "600" },
  progress: { flexDirection: "row", gap: 4, paddingHorizontal: 16, paddingBottom: 8, backgroundColor: "#fff" },
  bar: { flex: 1, height: 5, borderRadius: 3, backgroundColor: "#e5e7eb" },
  barOn: { backgroundColor: "#1f5bd8" },
  page: { padding: 16, gap: 12 },
  footer: { padding: 12, paddingBottom: Platform.OS === "ios" ? 28 : 12, backgroundColor: "#fff", borderTopWidth: 1, borderTopColor: "#e5e7eb" },
  h1: { fontSize: 28, fontWeight: "700", color: "#0f3d91" },
  h2: { fontSize: 16, fontWeight: "700", marginBottom: 4 },
  muted: { color: "#4b5563", fontSize: 15, fontWeight: "400" },
  small: { color: "#4b5563", fontSize: 13 },
  card: { backgroundColor: "#fff", borderRadius: 16, padding: 16, gap: 8, borderWidth: 1, borderColor: "#e5e7eb" },
  grid: { flexDirection: "row", flexWrap: "wrap", gap: 8, marginBottom: 8 },
  lang: { minWidth: "47%", flexGrow: 1, minHeight: 48, borderRadius: 12, borderWidth: 1, borderColor: "#d1d5db", alignItems: "center", justifyContent: "center", backgroundColor: "#fff" },
  langOn: { backgroundColor: "#1f5bd8", borderColor: "#1f5bd8" },
  langText: { fontSize: 16 },
  demo: { padding: 12, borderRadius: 12, borderWidth: 1, borderColor: "#e5e7eb" },
  btn: { minHeight: 48, borderRadius: 12, backgroundColor: "#1f5bd8", alignItems: "center", justifyContent: "center", paddingHorizontal: 16 },
  btnSecondary: { backgroundColor: "#fff", borderWidth: 1, borderColor: "#1f5bd8" },
  btnDisabled: { opacity: 0.5 },
  btnText: { color: "#fff", fontWeight: "700", fontSize: 16 },
  btnTextSecondary: { color: "#1748b0" },
  bubble: { maxWidth: "88%", padding: 12, borderRadius: 16, fontSize: 15, lineHeight: 22 },
  mine: { alignSelf: "flex-end", backgroundColor: "#1f5bd8", color: "#fff" },
  theirs: { alignSelf: "flex-start", backgroundColor: "#fff", color: "#111827", borderWidth: 1, borderColor: "#e5e7eb" },
  input: { flex: 1, minHeight: 48, borderWidth: 1, borderColor: "#d1d5db", borderRadius: 12, paddingHorizontal: 12, paddingVertical: 8, fontSize: 16, backgroundColor: "#fff" },
  price: { fontSize: 24, fontWeight: "700", color: "#0f3d91" },
  row: { flexDirection: "row", alignItems: "center", gap: 8, justifyContent: "space-between" },
  divider: { height: 1, backgroundColor: "#e5e7eb", marginVertical: 4 },
  check: { marginTop: 6 },
  capture: { minHeight: 48, borderRadius: 12, borderWidth: 2, borderStyle: "dashed", borderColor: "#1f5bd8", alignItems: "center", justifyContent: "center" },
  finding: { backgroundColor: "#fef3c7", borderRadius: 12, padding: 10, gap: 2 },
  error: { color: "#b91c1c", backgroundColor: "#fee2e2", padding: 10, borderRadius: 12 },
  dot: { width: 28, height: 28, borderRadius: 14, textAlign: "center", lineHeight: 28, backgroundColor: "#e5e7eb", color: "#6b7280", overflow: "hidden" },
  dotDone: { backgroundColor: "#16a34a", color: "#fff" },
  dotNow: { backgroundColor: "#1f5bd8", color: "#fff" },
  dotBlocked: { backgroundColor: "#f59e0b", color: "#fff" },
  wa: { backgroundColor: "#e7f8e9", padding: 10, borderRadius: 12, marginTop: 6 },
  waChannel: { fontSize: 11, fontWeight: "700", color: "#166534" },
});
