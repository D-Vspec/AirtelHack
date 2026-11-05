import { StatusBar } from "expo-status-bar";
import * as ImagePicker from "expo-image-picker";
import { useEffect, useRef, useState, type ComponentRef, type ReactNode } from "react";
import { ActivityIndicator, KeyboardAvoidingView, Platform, Pressable, ScrollView, StatusBar as RNStatusBar, StyleSheet, Text, TextInput, View } from "react-native";
import {
  accountText, createApiClient, DEMO_OWNERS, inr, KYC_MESSAGES, LANGUAGES, newLeadId, ORDER_STATUS_LABELS, REUSE_REASON_TEXT, t as translate,
  type AccountTextKey, type AccountView, type BusinessProfile, type KycApplication, type KycDocumentType, type KycFinding, type LangCode,
  type Order, type Quote, type ReuseStatus, type SentMessage, type StringKey, type Timeline,
} from "@udyam/shared";

// On a physical phone use the computer's LAN IP, e.g. EXPO_PUBLIC_API_URL=http://192.168.1.20:8080/api
// (or keep localhost and forward the port over USB: adb reverse tcp:8080 tcp:8080).
// A signed-in customer's session token (Authorization: Bearer). Kept in memory: signing in lasts until the app is closed.
let accountToken: string | null = null;
const api = createApiClient(process.env.EXPO_PUBLIC_API_URL ?? "http://localhost:8080/api", { accountToken: () => accountToken });

// Colour tokens from ts/apps/web/app/globals.css (the red/ink redesign).
const C = {
  brand50: "#fff3f3", brand100: "#ffe1e1", brand200: "#ffc2c2", brand600: "#e40000", brand700: "#b80000",
  ink50: "#f6f6f8", ink100: "#ececf0", ink200: "#dcdce3", ink500: "#6b6b76", ink700: "#3a3a42", ink900: "#16161a",
  mint50: "#ecfdf3", mint600: "#16a34a", amber50: "#fffbeb", amber200: "#fde68a", amber500: "#f59e0b", white: "#fff",
  wa: "#075e54", waBubble: "#d9fdd3", waBg: "#efeae2",
};
// Expo Go draws edge to edge on Android: keep content clear of the status and navigation bars.
const TOP_INSET = Platform.OS === "android" ? (RNStatusBar.currentHeight ?? 24) : Platform.OS === "ios" ? 48 : 0;
const BOTTOM_INSET = Platform.OS === "android" ? 20 : Platform.OS === "ios" ? 24 : 0;

// Quick replies in the owner's language, as on the web chat page (others use English).
const CHIPS: Partial<Record<LangCode, { ask: string[]; offer: string[] }>> = {
  en: { ask: ["We have GSTIN", "1 branch", "5 staff", "Only a mobile plan"], offer: ["Go ahead", "Too expensive", "Only a mobile plan"] },
  hi: { ask: ["GSTIN है", "एक ही दुकान", "5 लोग काम करते हैं", "सिर्फ़ मोबाइल प्लान"], offer: ["ठीक है, आगे बढ़ें", "बहुत महँगा है", "सिर्फ़ मोबाइल प्लान"] },
  ta: { ask: ["GSTIN உள்ளது", "ஒரே கிளை", "5 பேர் வேலை செய்கிறார்கள்", "மொபைல் பிளான் மட்டும்"], offer: ["சரி, தொடரலாம்", "விலை அதிகம்", "மொபைல் பிளான் மட்டும்"] },
};

type Screen = "home" | "chat" | "quote" | "consent" | "kyc" | "track" | "account";
const STEPS: Screen[] = ["chat", "quote", "consent", "kyc", "track"];
const STEP_KEYS: Record<Screen, StringKey> = { home: "appName", chat: "stepChat", quote: "stepQuote", consent: "stepConsent", kyc: "stepKyc", track: "stepTrack", account: "appName" };

interface Msg { role: "owner" | "advisor"; text: string }
// trackLead: a past order opened from the account screen (otherwise Track follows leadId).
interface Journey { lang: LangCode; leadId: string; sessionId?: string; messages: Msg[]; profile: BusinessProfile; quoteId?: string; specimen: string; draft: string; trackLead?: string }
interface Account { mobileMasked: string }
/** A fresh order: new lead ID, empty chat. */
const freshJourney = (prefix = "mobile", specimen = "K03", draft = ""): Partial<Journey> =>
  ({ leadId: newLeadId(prefix), sessionId: undefined, messages: [], profile: {}, quoteId: undefined, specimen, draft, trackLead: undefined });

const errText = (e: unknown, fallback: string) => (e as { detail?: string })?.detail || fallback;

export default function App() {
  const [screen, setScreen] = useState<Screen>("home");
  const [back, setBack] = useState<Screen>("home");
  const [j, setJ] = useState<Journey>({ lang: "en", leadId: "", messages: [], profile: {}, specimen: "K03", draft: "" });
  const [account, setAccount] = useState<Account | null>(null);
  const t = (k: StringKey, p?: Record<string, string | number>) => translate(j.lang, k, p);
  const update = (patch: Partial<Journey>) => setJ((cur) => ({ ...cur, ...patch }));
  const go = (to: Screen) => {
    if (to === "account" && screen !== "account") setBack(screen);
    setScreen(to);
  };
  const signIn = (token: string, a: Account) => { accountToken = token; setAccount(a); };
  const signOut = async () => {
    try { await api.accounts.logout(); } catch { /* already expired */ }
    accountToken = null;
    setAccount(null);
  };
  const props = { j, update, t, go, account, signIn, signOut, back };
  const step = STEPS.indexOf(screen);

  // A signed-in owner's orders join their account, so a later order can reuse the verified documents.
  useEffect(() => {
    if (account && j.leadId) api.accounts.linkLead(j.leadId).catch(() => { /* another account's order: stays a guest order */ });
  }, [account, j.leadId]);

  return (
    // "padding" on both platforms: with edge-to-edge Android the window no longer resizes for the keyboard.
    <KeyboardAvoidingView style={{ flex: 1, backgroundColor: C.ink50 }} behavior={Platform.OS === "web" ? undefined : "padding"}>
      <View style={s.header}>
        <Pressable style={s.brandRow} onPress={() => setScreen("home")} accessibilityRole="button">
          <View style={s.logo}><Text style={s.logoText}>U</Text></View>
          <Text style={s.brand} numberOfLines={1}>{t("appName")}</Text>
        </Pressable>
        <View style={s.headRight}>
          {step >= 0 && <View style={s.stepPill}><Text style={s.stepLabel} numberOfLines={1}>{step + 1}/5 · {t(STEP_KEYS[screen])}</Text></View>}
          <Pressable testID="account-link" accessibilityRole="button" accessibilityLabel={accountText(j.lang, "account")} onPress={() => go("account")} style={s.accountBtn}>
            <Text style={{ fontSize: 16 }}>👤</Text>
            {account && <View style={s.accountDot} />}
          </Pressable>
        </View>
      </View>
      {step >= 0 && (
        <View style={s.progress}>{STEPS.map((x, i) => <View key={x} style={[s.bar, i < step && s.barDone, i === step && s.barOn]} />)}</View>
      )}
      {screen === "home" && <Home {...props} />}
      {screen === "chat" && <Chat {...props} />}
      {screen === "quote" && <QuoteScreen {...props} />}
      {screen === "consent" && <Consent {...props} />}
      {screen === "kyc" && <Kyc {...props} />}
      {screen === "track" && <Track {...props} />}
      {screen === "account" && <AccountScreen {...props} />}
      <StatusBar style="dark" />
    </KeyboardAvoidingView>
  );
}

interface P {
  j: Journey; update: (p: Partial<Journey>) => void; t: (k: StringKey, p?: Record<string, string | number>) => string; go: (s: Screen) => void;
  account: Account | null; signIn: (token: string, a: Account) => void; signOut: () => Promise<void>; back: Screen;
}

function Btn({ title, onPress, disabled, variant = "primary", testID, busy }: { title: string; onPress: () => void; disabled?: boolean; variant?: "primary" | "secondary"; testID?: string; busy?: boolean }) {
  return (
    <Pressable testID={testID} accessibilityRole="button" disabled={disabled || busy} onPress={onPress}
      style={({ pressed }) => [s.btn, variant === "secondary" && s.btnSecondary, pressed && { opacity: 0.85 }, (disabled || busy) && s.btnDisabled]}>
      {busy ? <ActivityIndicator color={variant === "secondary" ? C.brand600 : C.white} /> : <Text style={[s.btnText, variant === "secondary" && s.btnTextSecondary]}>{title}</Text>}
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

const Avatar = () => <View style={s.avatar}><Text style={s.avatarText}>✦</Text></View>;
const AdvisorBubble = ({ text }: { text: string }) => (
  <View style={s.advisorRow}>
    <Avatar />
    <Text style={[s.bubble, s.theirs]}>{text}</Text>
  </View>
);

function Home({ j, update, t, go }: P) {
  const start = (lang: LangCode, prefix = "mobile", specimen = "K03", draft = "") => {
    update({ lang, ...freshJourney(prefix, specimen, draft) });
    go("chat");
  };
  return (
    <Page>
      <View style={s.hero}>
        <Text style={s.heroKicker}>Airtel · Accel Business</Text>
        <Text style={s.h1}>{t("appName")}</Text>
        <Text style={s.heroText}>{t("tagline")}</Text>
      </View>
      <Card>
        <Text style={s.h2}>{t("chooseLanguage")}</Text>
        <View style={s.grid}>
          {LANGUAGES.map((l) => (
            <Pressable key={l.code} onPress={() => update({ lang: l.code })} style={[s.lang, j.lang === l.code && s.langOn]}>
              <Text style={[s.langText, j.lang === l.code && { color: C.white, fontWeight: "700" }]}>{l.native}</Text>
            </Pressable>
          ))}
        </View>
        <Btn title={`${t("start")} →`} onPress={() => start(j.lang)} />
      </Card>
      <Card>
        <Text style={s.h2}>{t("demoOwners")}</Text>
        {DEMO_OWNERS.map((o) => (
          <Pressable key={o.id} testID={`demo-${o.id}`} onPress={() => start(o.language, o.leadPrefix, o.specimen, o.firstMessage)} style={s.demo}>
            <Text style={{ fontWeight: "600", color: C.ink900 }}>{o.title}</Text>
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

  const chips = (CHIPS[j.lang] ?? CHIPS.en)!;
  return (
    <View style={{ flex: 1 }}>
      <ScrollView ref={scroll} onContentSizeChange={() => scroll.current?.scrollToEnd({ animated: true })} contentContainerStyle={s.page} keyboardShouldPersistTaps="handled">
        {/* the advisor speaks first, as on the web */}
        <AdvisorBubble text={t("tagline")} />
        {j.messages.map((m, i) => (m.role === "owner"
          ? <Text key={i} style={[s.bubble, s.mine]}>{m.text}</Text>
          : <AdvisorBubble key={i} text={m.text} />))}
        {busy && (
          <View style={s.advisorRow}>
            <Avatar />
            <View style={[s.bubble, s.theirs, { paddingVertical: 14 }]}><ActivityIndicator size="small" color={C.ink500} /></View>
          </View>
        )}
        <ErrorBox text={error} />
        {quote && (
          <View style={[s.card, s.flush]}>
            <View style={s.darkHead}>
              <Text style={s.kicker}>{t("stepQuote")}</Text>
              <Text style={s.bigPrice}>{inr(quote.monthlyTotalInr)} <Text style={s.per}>{t("perMonth")}</Text></Text>
            </View>
            <Text style={[s.small, { padding: 14 }]}>{inr(quote.oneTimeTotalInr)} {t("oneTime")} · {t("installIn", { days: quote.expectedInstallDays })}</Text>
          </View>
        )}
      </ScrollView>
      <View style={s.footer}>
        {quote ? <Btn testID="see-quote" title={`${t("seeQuote")} →`} onPress={() => go("quote")} /> : null}
        <ScrollView horizontal showsHorizontalScrollIndicator={false} keyboardShouldPersistTaps="handled" contentContainerStyle={s.chips} style={{ marginTop: quote ? 8 : 0, flexGrow: 0 }}>
          {(quote ? chips.offer : chips.ask).map((c) => (
            <Pressable key={c} disabled={busy} onPress={() => send(c)} style={[s.chip, busy && s.btnDisabled]}><Text style={s.chipText}>{c}</Text></Pressable>
          ))}
        </ScrollView>
        <View style={s.composer}>
          <TextInput testID="msg" value={input} onChangeText={setInput} placeholder={t("chatPlaceholder")} placeholderTextColor={C.ink500} multiline style={s.input} />
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
      {!quote && !error && <ActivityIndicator color={C.brand600} />}
      <ErrorBox text={error} />
      {quote && (
        <View style={[s.card, s.flush]}>
          <View style={s.darkHead}>
            <Text style={s.kicker}>{t("stepQuote")}</Text>
            <Text style={s.bigPrice}>{inr(quote.monthlyTotalInr)} <Text style={s.per}>{t("perMonth")}</Text></Text>
            <View style={s.pillBrand}><Text style={s.pillBrandText}>{inr(quote.oneTimeTotalInr)} {t("oneTime")}</Text></View>
          </View>
          <View style={{ paddingHorizontal: 16, paddingVertical: 4 }}>
            {quote.lines.map((l, i) => (
              <View key={l.productId} style={[s.row, s.lineRow, i === 0 && { borderTopWidth: 0 }]}>
                <Text style={{ flex: 1, color: C.ink900 }}>{l.productName}{l.quantity > 1 ? ` × ${l.quantity}` : ""}</Text>
                <Text style={{ fontWeight: "600", color: C.ink900 }}>{inr(l.monthlyInr)}</Text>
              </View>
            ))}
          </View>
          <View style={s.cardFoot}>
            <Text style={s.small}>{t("totalContract", { months: quote.contractMonths })}: {inr(quote.totalContractValueInr)}</Text>
            <Text style={s.small}>{t("installIn", { days: quote.expectedInstallDays })}</Text>
          </View>
        </View>
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
        <Text style={{ color: C.ink700, lineHeight: 21 }}>{t("consentBody")}</Text>
        {(["consentKyc", "consentProvision", "consentComms"] as StringKey[]).map((k) => (
          <View key={k} style={s.checkRow}>
            <View style={s.checkBox}><Text style={s.checkMark}>✓</Text></View>
            <Text style={{ flex: 1, color: C.ink900 }}>{t(k)}</Text>
          </View>
        ))}
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
    <View style={[s.finding, f.route === "reject" && s.findingReject]}>
      <Text style={{ fontWeight: "600", color: C.ink900 }}>{m?.message ?? f.code}</Text>
      {m?.action && <Text style={s.small}>→ {m.action}</Text>}
    </View>
  );
}

function Kyc({ j, t, go, update, account }: P) {
  const [have, setHave] = useState<Partial<Record<KycDocumentType, KycFinding[]>>>({});
  const [busy, setBusy] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [result, setResult] = useState<KycApplication | null>(null);
  // returning customer: reuse the account's earlier verification (go/pkg/kyc ReusePolicy decides)
  const [reuse, setReuse] = useState<ReuseStatus | null>(null);
  const [unchanged, setUnchanged] = useState(false);
  const [refused, setRefused] = useState<string[] | null>(null);
  const [reuseHidden, setReuseHidden] = useState(false);
  const A = (k: AccountTextKey) => accountText(j.lang, k);

  useEffect(() => {
    if (!account || !j.leadId) { setReuse(null); return; }
    api.accounts.reuseStatus(j.leadId).then(setReuse).catch(() => setReuse(null));
  }, [account, j.leadId]);

  async function useVerified() {
    setBusy("reuse");
    setError(null);
    try {
      setResult(await api.accounts.reuse(j.leadId, unchanged));
      setReuseHidden(true);
    } catch (e) {
      const reasons = (e as { body?: { reasons?: string[] } }).body?.reasons;
      if (reasons) setRefused(reasons);
      else setError(errText(e, t("error")));
    } finally {
      setBusy(null);
    }
  }

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
        {canTrack && <Btn testID="go-track" title={`${t("stepTrack")} →`} onPress={() => { update({ trackLead: undefined }); go("track"); }} />}
      </View>
    }>
      {reuse?.reusable && reuse.verified && !result && !reuseHidden && (
        <View testID="reuse-card" style={[s.card, { borderColor: C.mint600, borderWidth: 2 }]}>
          <Text style={s.h2}>🛡️ {A("reuseTitle")}</Text>
          <Text style={{ color: C.ink700, lineHeight: 21 }}>{A("reuseBody")}</Text>
          <View style={s.verifiedBox}>
            <Text style={{ fontWeight: "700", color: C.ink900 }}>{reuse.verified.tradeName || reuse.verified.legalName} · GSTIN {reuse.verified.gstinMasked}</Text>
            {!!reuse.verified.address && <Text style={s.small}>{reuse.verified.address}</Text>}
            <Text style={s.small}>{A("verifiedOn")} {new Date(reuse.verified.verifiedAt).toLocaleDateString()}</Text>
          </View>
          <Pressable testID="unchanged" accessibilityRole="checkbox" accessibilityState={{ checked: unchanged }} onPress={() => setUnchanged((u) => !u)} style={s.checkRow}>
            <View style={[s.checkBox, !unchanged && s.checkBoxOff]}>{unchanged && <Text style={s.checkMark}>✓</Text>}</View>
            <Text style={{ flex: 1, color: C.ink900 }}>{A("unchanged")}</Text>
          </Pressable>
          {refused && (
            <View style={s.finding}>
              <Text style={{ fontWeight: "600", color: C.ink900 }}>{A("reuseNotPossible")}</Text>
              {refused.map((r) => <Text key={r} style={s.small}>• {REUSE_REASON_TEXT[r as keyof typeof REUSE_REASON_TEXT] ?? r}</Text>)}
            </View>
          )}
          <Btn testID="reuse-kyc" title={`✓ ${A("reuseButton")}`} disabled={!unchanged || (!!busy && busy !== "reuse")} busy={busy === "reuse"} onPress={useVerified} />
          <Pressable onPress={() => setReuseHidden(true)}><Text style={s.link}>{A("somethingChanged")}</Text></Pressable>
        </View>
      )}
      {reuse && !reuse.reusable && reuse.verified && !result && (
        <View style={s.finding}>
          <Text style={{ fontWeight: "600", color: C.ink900 }}>{A("reuseNotPossible")}</Text>
          {reuse.reasons.map((r) => <Text key={r} style={s.small}>• {REUSE_REASON_TEXT[r]}</Text>)}
        </View>
      )}
      {!account && !result && (
        <Pressable testID="sign-in-to-skip" onPress={() => go("account")}><Text style={s.link}>👤 {A("signInToSkip")}</Text></Pressable>
      )}
      {DOCS.map(({ type, label }) => (
        <Card key={type}>
          <View style={s.row}>
            <Text style={[s.h2, { flex: 1 }]}>{t(label)}</Text>
            {(have[type] || status === "pass") && <View style={s.pillMint}><Text style={s.pillMintText}>✓ {t("uploaded")}</Text></View>}
          </View>
          <Pressable onPress={() => capture(type)} disabled={!!busy} style={s.capture}>
            {busy === type ? <ActivityIndicator color={C.brand600} /> : <Text style={{ color: C.brand700, fontWeight: "600" }}>📷 {t("takePhoto")}</Text>}
          </Pressable>
          {[...(have[type] ?? []), ...(result?.findings ?? []).filter((f) => f.document === type)].map((f, i) => <FindingRow key={i} f={f} lang={j.lang} />)}
        </Card>
      ))}
      <Btn testID="use-sample" title={`${t("useSample")} (${j.specimen})`} variant="secondary" busy={busy === "sample"} onPress={sample} />
      <ErrorBox text={error} />
      {statusText && (
        <View testID="kyc-result" style={[s.finding, status === "pass" && s.findingPass, status === "reject" && s.findingReject]}>
          <Text style={{ fontWeight: "700", color: C.ink900 }}>{statusText}</Text>
          <Text style={{ display: "none" }}>{status}</Text>
        </View>
      )}
    </Page>
  );
}

function Track({ j, t }: P) {
  const [tl, setTl] = useState<Timeline | null>(null);
  const [msgs, setMsgs] = useState<SentMessage[]>([]);
  const lead = j.trackLead ?? j.leadId;
  useEffect(() => {
    let stop = false;
    setTl(null);
    setMsgs([]);
    const poll = async () => {
      try {
        const { orders } = await api.tracker.orders({ leadId: lead });
        if (orders[0]) setTl(await api.tracker.timeline(orders[0].id));
        setMsgs((await api.notify.messages(lead)).messages);
      } catch { /* retry next tick */ }
      if (!stop) setTimeout(poll, 3000);
    };
    void poll();
    return () => { stop = true; };
  }, [lead]);

  const labels = ORDER_STATUS_LABELS[j.lang];
  const o = tl?.order;
  const live = o?.status === "activated";
  return (
    <Page>
      {!o && <Text style={{ color: C.ink700 }}>{t("noOrder")}</Text>}
      {o && (
        <View style={[s.card, s.flush]}>
          <View style={[s.darkHead, live && { backgroundColor: C.mint600 }]}>
            <Text style={s.kicker}>#{o.id.replace(/-/g, "").slice(0, 8).toUpperCase()}</Text>
            <Text testID="order-status" style={s.trackStatus}>{labels[o.status] ?? o.status}</Text>
            <Text testID={`order-${o.status}`} style={{ display: "none" }}>{o.status}</Text>
            {o.expectedActivationDate && !live && <Text style={{ color: "rgba(255,255,255,0.8)" }}>{t("expectedBy", { date: o.expectedActivationDate })}</Text>}
          </View>
          <View style={{ padding: 16 }}>
            {tl!.steps.map((st, i) => (
              <View key={st.status} style={s.stepRow}>
                <View style={{ alignItems: "center" }}>
                  <Text style={[s.dot, st.state === "done" && s.dotDone, st.state === "current" && s.dotNow, st.state === "blocked" && s.dotBlocked]}>
                    {st.state === "done" ? "✓" : st.state === "blocked" ? "!" : String(i + 1)}
                  </Text>
                  {i < tl!.steps.length - 1 && <View style={[s.stepLine, st.state === "done" && { backgroundColor: C.ink900 }]} />}
                </View>
                <Text style={{ flex: 1, paddingTop: 6, paddingBottom: 12, color: st.state === "upcoming" ? C.ink500 : C.ink900, fontWeight: st.state === "upcoming" ? "400" : "600" }}>{labels[st.status]}</Text>
              </View>
            ))}
          </View>
        </View>
      )}
      {msgs.length > 0 && (
        <View style={[s.card, s.flush]}>
          <View style={s.waHead}><Text style={{ color: C.white, fontWeight: "600" }}>💬 {t("messagesSent")}</Text></View>
          <View style={{ backgroundColor: C.waBg, padding: 10, gap: 8 }}>
            {msgs.map((m) => (
              <View key={m.id} style={s.wa}><Text style={s.waChannel}>{m.channel.toUpperCase()}</Text><Text style={{ color: C.ink900 }}>{m.body}</Text></View>
            ))}
          </View>
        </View>
      )}
    </Page>
  );
}

interface Row { leadId: string; reusedKyc: boolean; order: Order | null }
const NEEDS_KYC = new Set(["quote_accepted", "kyc_pending"]);
const digits = (v: string) => v.replace(/\D/g, "");

/** Customer account: sign in with mobile + one-time code, every past order with tracking, new order (same API as the web /account). */
function AccountScreen({ j, update, t, go, account, signIn, signOut, back }: P) {
  const A = (k: AccountTextKey) => accountText(j.lang, k);
  const [mobile, setMobile] = useState("");
  const [code, setCode] = useState("");
  const [sent, setSent] = useState<{ mobileMasked: string; demoCode?: string } | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [view, setView] = useState<AccountView | null>(null);
  const [rows, setRows] = useState<Row[] | null>(null);

  useEffect(() => {
    if (!account) { setView(null); setRows(null); return; }
    let live = true;
    (async () => {
      try {
        const v = await api.accounts.me();
        const r = await Promise.all(v.leads.map(async (l) => {
          const { orders } = await api.tracker.orders({ leadId: l.leadId }).catch(() => ({ orders: [] as Order[] }));
          return { leadId: l.leadId, reusedKyc: l.reusedKyc, order: orders[0] ?? null };
        }));
        if (live) { setView(v); setRows(r); }
      } catch (e) {
        if ((e as { status?: number }).status === 401) await signOut();
        else if (live) setError(errText(e, t("error")));
      }
    })();
    return () => { live = false; };
  }, [account]); // eslint-disable-line react-hooks/exhaustive-deps

  async function sendCode() {
    setBusy(true);
    setError(null);
    try {
      setSent(await api.accounts.requestCode(digits(mobile)));
    } catch (e) {
      setError(errText(e, t("error")));
    } finally {
      setBusy(false);
    }
  }

  async function verify() {
    setBusy(true);
    setError(null);
    try {
      // the order in progress (if any) joins the account
      const r = await api.accounts.verify({ mobile: digits(mobile), code, leadId: j.leadId || undefined });
      signIn(r.token, { mobileMasked: r.account.mobileMasked });
      setSent(null);
      setCode("");
    } catch (e) {
      setError(errText(e, t("error")));
    } finally {
      setBusy(false);
    }
  }

  const STATUS = ORDER_STATUS_LABELS[j.lang];
  const orders = (rows ?? []).filter((r) => r.order);
  return (
    <Page footer={account && <Btn testID="new-order" title={`${A("newOrder")} →`} onPress={() => { update(freshJourney()); go("chat"); }} />}>
      <Pressable onPress={() => go(back === "account" ? "home" : back)}><Text style={s.link}>← {A("back")}</Text></Pressable>
      <Text style={s.title}>{A("account")}</Text>

      {!account && (
        <Card>
          <Text style={s.h2}>👤 {A("signInTitle")}</Text>
          <Text style={{ color: C.ink700, lineHeight: 21 }}>{A("signInBody")}</Text>
          {!sent ? (
            <>
              <Text style={s.label}>{A("mobile")}</Text>
              <View style={s.field}>
                <Text style={{ color: C.ink500, fontSize: 16 }}>+91</Text>
                <TextInput testID="mobile" value={mobile} onChangeText={(v) => setMobile(v.replace(/[^0-9 ]/g, ""))} keyboardType="phone-pad"
                  autoComplete="tel" maxLength={12} placeholder="98765 43210" placeholderTextColor={C.ink500} style={s.fieldInput} />
              </View>
              <Btn testID="send-code" title={A("sendCode")} busy={busy} disabled={digits(mobile).length !== 10} onPress={sendCode} />
            </>
          ) : (
            <>
              {sent.demoCode && (
                <View style={s.infoBox}><Text style={{ color: C.ink900 }}>{A("demoCode")} <Text testID="demo-code" style={s.code}>{sent.demoCode}</Text></Text></View>
              )}
              <Text style={s.label}>{A("code")} · +91 {sent.mobileMasked}</Text>
              <TextInput testID="otp" value={code} onChangeText={(v) => setCode(digits(v))} keyboardType="number-pad" autoComplete="sms-otp"
                maxLength={6} style={[s.field, s.otp]} />
              <Btn testID="sign-in" title={A("signIn")} busy={busy} disabled={code.length !== 6} onPress={verify} />
              <Pressable onPress={() => { setSent(null); setCode(""); }}><Text style={s.link}>{A("changeNumber")}</Text></Pressable>
            </>
          )}
        </Card>
      )}

      <ErrorBox text={error} />

      {account && (
        <>
          <Card>
            <View style={s.row}>
              <Text style={{ flex: 1, color: C.ink900 }}>👤 {A("signedInAs")} <Text testID="signed-in" style={{ fontWeight: "700" }}>+91 {account.mobileMasked}</Text></Text>
              <Pressable testID="sign-out" onPress={() => void signOut()}><Text style={s.link}>{A("signOut")}</Text></Pressable>
            </View>
          </Card>

          <Card>
            <Text style={s.h2}>🛡️ {A("verified")}</Text>
            {!view && <ActivityIndicator color={C.brand600} />}
            {view && !view.verified && <Text style={{ color: C.ink700 }}>{A("notVerified")}</Text>}
            {view?.verified && (
              <View testID="verified-business" style={{ gap: 2 }}>
                <Text style={{ fontWeight: "700", fontSize: 16, color: C.ink900 }}>{view.verified.tradeName || view.verified.legalName}</Text>
                {!!view.verified.legalName && <Text style={s.small}>{view.verified.legalName}</Text>}
                {!!view.verified.address && <Text style={s.small}>{view.verified.address}</Text>}
                <View style={s.pills}>
                  <View style={s.pillMint}><Text style={s.pillMintText}>✓ GSTIN {view.verified.gstinMasked}</Text></View>
                  <View style={s.pillInk}><Text style={s.pillInkText}>{A("verifiedOn")} {new Date(view.verified.verifiedAt).toLocaleDateString()}</Text></View>
                </View>
              </View>
            )}
          </Card>

          <View style={[s.card, s.flush]}>
            <Text style={[s.h2, { paddingHorizontal: 16, paddingTop: 14 }]}>🚚 {A("orders")}</Text>
            {!rows && <ActivityIndicator style={{ padding: 16 }} color={C.brand600} />}
            {rows && orders.length === 0 && <Text style={{ color: C.ink700, padding: 16, paddingTop: 4 }}>{A("noOrders")}</Text>}
            {orders.map(({ leadId, order, reusedKyc }) => {
              const o = order!;
              const done = o.status === "activated";
              return (
                <View key={leadId} testID="order-row" style={s.orderRow}>
                  <View style={s.pills}>
                    <Text style={{ fontWeight: "700", color: C.ink900 }}>#{o.id.replace(/-/g, "").slice(0, 8).toUpperCase()}</Text>
                    <View style={done ? s.pillMint : s.pillInk}><Text style={done ? s.pillMintText : s.pillInkText}>{STATUS[o.status] ?? o.status}</Text></View>
                    {leadId === j.leadId && <View style={s.pillBrand}><Text style={s.pillBrandText}>{A("current")}</Text></View>}
                    {reusedKyc && <View style={s.pillInk}><Text style={s.pillInkText}>{A("reusedKyc")}</Text></View>}
                  </View>
                  <Text style={s.small} numberOfLines={1}>{o.lines.map((l) => l.productName).join(" · ")}</Text>
                  <Text style={{ color: C.ink700, fontSize: 13 }}>{inr(o.monthlyTotalInr)} {t("perMonth")} · {new Date(o.createdAt).toLocaleDateString()}</Text>
                  <View style={{ flexDirection: "row", gap: 8, marginTop: 6 }}>
                    {NEEDS_KYC.has(o.status) && (
                      <View style={{ flex: 1 }}>
                        <Btn testID="finish-kyc" title={A("finishKyc")} onPress={() => { update({ leadId, trackLead: undefined }); go("kyc"); }} />
                      </View>
                    )}
                    <View style={{ flex: 1 }}>
                      <Btn testID="track-order" variant="secondary" title={`${A("track")} →`} onPress={() => { update({ trackLead: leadId }); go("track"); }} />
                    </View>
                  </View>
                </View>
              );
            })}
          </View>
        </>
      )}
    </Page>
  );
}

const s = StyleSheet.create({
  headRight: { flexDirection: "row", alignItems: "center", gap: 8, flexShrink: 1 },
  accountBtn: { width: 40, height: 40, borderRadius: 20, backgroundColor: C.ink50, borderWidth: 1, borderColor: C.ink100, alignItems: "center", justifyContent: "center" },
  accountDot: { position: "absolute", top: 0, right: 0, width: 11, height: 11, borderRadius: 6, backgroundColor: C.mint600, borderWidth: 2, borderColor: C.white },
  title: { fontSize: 24, fontWeight: "800", color: C.ink900 },
  link: { color: C.ink500, textDecorationLine: "underline", fontSize: 14, paddingVertical: 4 },
  label: { fontWeight: "600", color: C.ink900, marginTop: 6 },
  field: { flexDirection: "row", alignItems: "center", gap: 6, minHeight: 50, borderRadius: 14, backgroundColor: C.ink50, borderWidth: 1, borderColor: C.ink200, paddingHorizontal: 14 },
  fieldInput: { flex: 1, minHeight: 48, fontSize: 17, letterSpacing: 1, color: C.ink900 },
  otp: { fontSize: 22, letterSpacing: 8, fontWeight: "700", color: C.ink900, textAlign: "center" },
  code: { fontWeight: "800", letterSpacing: 3, fontSize: 16 },
  infoBox: { backgroundColor: C.brand50, borderRadius: 14, padding: 12, borderWidth: 1, borderColor: C.brand100 },
  verifiedBox: { backgroundColor: C.mint50, borderRadius: 14, padding: 12, gap: 2 },
  checkBoxOff: { backgroundColor: C.white, borderWidth: 2, borderColor: C.ink200 },
  orderRow: { borderTopWidth: 1, borderTopColor: C.ink100, padding: 16, gap: 4 },
  pills: { flexDirection: "row", flexWrap: "wrap", alignItems: "center", gap: 6 },
  pillInk: { backgroundColor: C.ink100, borderRadius: 999, paddingHorizontal: 10, paddingVertical: 3 },
  pillInkText: { color: C.ink700, fontSize: 12, fontWeight: "600" },
  header: { paddingTop: TOP_INSET + 10, paddingHorizontal: 16, paddingBottom: 10, backgroundColor: C.white, flexDirection: "row", justifyContent: "space-between", alignItems: "center", gap: 8 },
  brandRow: { flexDirection: "row", alignItems: "center", gap: 8, flexShrink: 1 },
  logo: { width: 30, height: 30, borderRadius: 10, backgroundColor: C.brand600, alignItems: "center", justifyContent: "center" },
  logoText: { color: C.white, fontWeight: "800", fontSize: 16 },
  brand: { fontSize: 18, fontWeight: "800", color: C.ink900, flexShrink: 1 },
  stepPill: { backgroundColor: C.ink50, borderRadius: 999, paddingHorizontal: 10, paddingVertical: 4, borderWidth: 1, borderColor: C.ink100, flexShrink: 1 },
  stepLabel: { color: C.ink700, fontWeight: "600", fontSize: 13 },
  progress: { flexDirection: "row", gap: 4, paddingHorizontal: 16, paddingBottom: 10, backgroundColor: C.white, borderBottomWidth: 1, borderBottomColor: C.ink100 },
  bar: { flex: 1, height: 4, borderRadius: 2, backgroundColor: C.ink100 },
  barDone: { backgroundColor: C.ink900 },
  barOn: { backgroundColor: C.brand600 },
  page: { padding: 16, gap: 12 },
  footer: { padding: 12, paddingBottom: 12 + BOTTOM_INSET, backgroundColor: C.white, borderTopWidth: 1, borderTopColor: C.ink100 },
  hero: { backgroundColor: C.ink900, borderRadius: 20, padding: 20, gap: 6 },
  heroKicker: { color: C.brand200, fontSize: 12, fontWeight: "700", letterSpacing: 1, textTransform: "uppercase" },
  h1: { fontSize: 30, fontWeight: "800", color: C.white, letterSpacing: -0.5 },
  heroText: { color: "rgba(255,255,255,0.8)", fontSize: 15, lineHeight: 21 },
  h2: { fontSize: 16, fontWeight: "700", color: C.ink900, marginBottom: 4 },
  small: { color: C.ink500, fontSize: 13 },
  card: { backgroundColor: C.white, borderRadius: 20, padding: 16, gap: 8, borderWidth: 1, borderColor: C.ink100, shadowColor: C.ink900, shadowOpacity: 0.06, shadowRadius: 12, shadowOffset: { width: 0, height: 4 }, elevation: 1 },
  flush: { padding: 0, gap: 0, overflow: "hidden" },
  cardFoot: { borderTopWidth: 1, borderTopColor: C.ink100, paddingHorizontal: 16, paddingVertical: 12, gap: 2 },
  grid: { flexDirection: "row", flexWrap: "wrap", gap: 8, marginBottom: 8 },
  lang: { minWidth: "47%", flexGrow: 1, minHeight: 48, borderRadius: 14, borderWidth: 1, borderColor: C.ink200, alignItems: "center", justifyContent: "center", backgroundColor: C.white },
  langOn: { backgroundColor: C.brand600, borderColor: C.brand600 },
  langText: { fontSize: 16, color: C.ink900 },
  demo: { padding: 12, borderRadius: 14, borderWidth: 1, borderColor: C.ink100, backgroundColor: C.ink50 },
  btn: { minHeight: 48, borderRadius: 14, backgroundColor: C.brand600, alignItems: "center", justifyContent: "center", paddingHorizontal: 18 },
  btnSecondary: { backgroundColor: C.white, borderWidth: 1, borderColor: C.ink200 },
  btnDisabled: { opacity: 0.45 },
  btnText: { color: C.white, fontWeight: "700", fontSize: 16 },
  btnTextSecondary: { color: C.ink900 },
  advisorRow: { flexDirection: "row", alignItems: "flex-start", gap: 8, maxWidth: "92%" },
  avatar: { width: 32, height: 32, borderRadius: 16, backgroundColor: C.ink900, alignItems: "center", justifyContent: "center" },
  avatarText: { color: C.white, fontSize: 14 },
  bubble: { paddingHorizontal: 14, paddingVertical: 10, borderRadius: 18, fontSize: 15, lineHeight: 22, overflow: "hidden" },
  mine: { alignSelf: "flex-end", maxWidth: "85%", backgroundColor: C.brand600, color: C.white, borderTopRightRadius: 6 },
  theirs: { flexShrink: 1, backgroundColor: C.white, color: C.ink900, borderWidth: 1, borderColor: C.ink100, borderTopLeftRadius: 6 },
  chips: { gap: 8, paddingBottom: 8 },
  chip: { minHeight: 36, borderRadius: 999, paddingHorizontal: 14, justifyContent: "center", backgroundColor: C.white, borderWidth: 1, borderColor: C.ink200 },
  chipText: { color: C.ink900, fontWeight: "500", fontSize: 14 },
  composer: { flexDirection: "row", alignItems: "flex-end", gap: 8, backgroundColor: C.ink50, borderRadius: 18, padding: 6, borderWidth: 1, borderColor: C.ink100 },
  input: { flex: 1, minHeight: 44, maxHeight: 120, paddingHorizontal: 10, paddingVertical: 10, fontSize: 16, color: C.ink900 },
  darkHead: { backgroundColor: C.ink900, padding: 16, gap: 4 },
  kicker: { color: "rgba(255,255,255,0.7)", fontSize: 12, letterSpacing: 1 },
  bigPrice: { color: C.white, fontSize: 28, fontWeight: "800" },
  per: { color: "rgba(255,255,255,0.7)", fontSize: 15, fontWeight: "400" },
  lineRow: { paddingVertical: 10, borderTopWidth: 1, borderTopColor: C.ink100 },
  pillBrand: { alignSelf: "flex-start", backgroundColor: C.brand100, borderRadius: 999, paddingHorizontal: 10, paddingVertical: 3, marginTop: 4 },
  pillBrandText: { color: C.brand700, fontSize: 12, fontWeight: "600" },
  pillMint: { backgroundColor: C.mint50, borderRadius: 999, paddingHorizontal: 10, paddingVertical: 3 },
  pillMintText: { color: C.mint600, fontSize: 12, fontWeight: "700" },
  row: { flexDirection: "row", alignItems: "center", gap: 8, justifyContent: "space-between" },
  checkRow: { flexDirection: "row", alignItems: "center", gap: 10, marginTop: 8 },
  checkBox: { width: 22, height: 22, borderRadius: 6, backgroundColor: C.brand600, alignItems: "center", justifyContent: "center" },
  checkMark: { color: C.white, fontWeight: "800", fontSize: 13 },
  capture: { minHeight: 52, borderRadius: 14, borderWidth: 2, borderStyle: "dashed", borderColor: C.brand200, backgroundColor: C.brand50, alignItems: "center", justifyContent: "center" },
  finding: { backgroundColor: C.amber50, borderRadius: 14, padding: 12, gap: 2, borderWidth: 1, borderColor: C.amber200 },
  findingReject: { backgroundColor: C.brand50, borderColor: C.brand200 },
  findingPass: { backgroundColor: C.mint50, borderColor: "#bbf7d0" },
  error: { color: C.brand700, backgroundColor: C.brand50, padding: 12, borderRadius: 14, borderWidth: 1, borderColor: C.brand200, overflow: "hidden" },
  trackStatus: { color: C.white, fontSize: 24, fontWeight: "800" },
  stepRow: { flexDirection: "row", gap: 12 },
  stepLine: { width: 2, flex: 1, minHeight: 10, backgroundColor: C.ink200, marginVertical: 2 },
  dot: { width: 32, height: 32, borderRadius: 16, textAlign: "center", lineHeight: 32, backgroundColor: C.ink100, color: C.ink500, overflow: "hidden", fontWeight: "700" },
  dotDone: { backgroundColor: C.ink900, color: C.white },
  dotNow: { backgroundColor: C.brand600, color: C.white },
  dotBlocked: { backgroundColor: C.amber500, color: C.white },
  waHead: { backgroundColor: C.wa, paddingHorizontal: 14, paddingVertical: 10 },
  wa: { backgroundColor: C.waBubble, padding: 10, borderRadius: 14, borderTopRightRadius: 4, marginLeft: 16 },
  waChannel: { fontSize: 10, fontWeight: "700", color: C.wa, letterSpacing: 1 },
});
