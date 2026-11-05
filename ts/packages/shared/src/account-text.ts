import type { LangCode } from "./i18n";

// Account screens in English, Hindi and Tamil (the pilot languages); others fall back to English.
const EN = {
  account: "My account", signInTitle: "Sign in to see your orders", signInBody: "Ordered before? Sign in with your mobile number to track every order and skip the documents next time.",
  mobile: "Mobile number", sendCode: "Send code", code: "6-digit code", signIn: "Sign in", demoCode: "Demo: no SMS is sent. Your code is", changeNumber: "Change number",
  signedInAs: "Signed in as", signOut: "Sign out", orders: "My orders", noOrders: "No orders yet.", track: "Track", newOrder: "Start a new order",
  verified: "Verified business", notVerified: "No verified documents yet. They are saved after your first KYC check.", verifiedOn: "Verified on",
  reuseTitle: "Use your verified documents", reuseBody: "We verified your business before. If nothing has changed, you don't need to upload anything.",
  unchanged: "Nothing has changed: business name, address, GSTIN and authorised signatory are the same.", reuseButton: "Use verified details", somethingChanged: "Something changed? Upload new documents below.",
  reuseNotPossible: "We need fresh documents:", signInToSkip: "Ordered before? Sign in to skip the documents.", current: "Current", reusedKyc: "KYC reused",
  back: "Back", finishKyc: "Finish verification",
};
export type AccountTextKey = keyof typeof EN;
const HI: Partial<Record<AccountTextKey, string>> = {
  account: "मेरा खाता", signInTitle: "अपने ऑर्डर देखने के लिए साइन इन करें", signInBody: "पहले ऑर्डर किया है? मोबाइल नंबर से साइन इन करें: हर ऑर्डर ट्रैक करें और अगली बार दस्तावेज़ दोबारा न दें।",
  mobile: "मोबाइल नंबर", sendCode: "कोड भेजें", code: "6 अंकों का कोड", signIn: "साइन इन", demoCode: "डेमो: SMS नहीं भेजा जाता। आपका कोड है", changeNumber: "नंबर बदलें",
  signedInAs: "साइन इन:", signOut: "साइन आउट", orders: "मेरे ऑर्डर", noOrders: "अभी कोई ऑर्डर नहीं।", track: "ट्रैक करें", newOrder: "नया ऑर्डर शुरू करें",
  verified: "सत्यापित व्यवसाय", notVerified: "अभी कोई सत्यापित दस्तावेज़ नहीं। पहली KYC के बाद ये सहेजे जाते हैं।", verifiedOn: "सत्यापन तिथि",
  reuseTitle: "अपने सत्यापित दस्तावेज़ इस्तेमाल करें", reuseBody: "हम आपका व्यवसाय पहले सत्यापित कर चुके हैं। कुछ नहीं बदला है तो कुछ भी अपलोड नहीं करना है।",
  unchanged: "कुछ नहीं बदला: व्यवसाय का नाम, पता, GSTIN और अधिकृत हस्ताक्षरकर्ता वही हैं।", reuseButton: "सत्यापित विवरण इस्तेमाल करें", somethingChanged: "कुछ बदला है? नीचे नए दस्तावेज़ अपलोड करें।",
  reuseNotPossible: "नए दस्तावेज़ चाहिए:", signInToSkip: "पहले ऑर्डर किया है? दस्तावेज़ छोड़ने के लिए साइन इन करें।", current: "मौजूदा", reusedKyc: "KYC दोबारा इस्तेमाल", back: "वापस", finishKyc: "सत्यापन पूरा करें",
};
const TA: Partial<Record<AccountTextKey, string>> = {
  account: "என் கணக்கு", signInTitle: "உங்கள் ஆர்டர்களைப் பார்க்க உள்நுழையவும்", signInBody: "முன்பு ஆர்டர் செய்தீர்களா? மொபைல் எண்ணுடன் உள்நுழைந்து ஒவ்வொரு ஆர்டரையும் கண்காணிக்கவும்; அடுத்த முறை ஆவணங்கள் தேவையில்லை.",
  mobile: "மொபைல் எண்", sendCode: "குறியீடு அனுப்பு", code: "6 இலக்க குறியீடு", signIn: "உள்நுழை", demoCode: "டெமோ: SMS அனுப்பப்படாது. உங்கள் குறியீடு", changeNumber: "எண்ணை மாற்று",
  signedInAs: "உள்நுழைந்தவர்", signOut: "வெளியேறு", orders: "என் ஆர்டர்கள்", noOrders: "இன்னும் ஆர்டர் இல்லை.", track: "கண்காணி", newOrder: "புதிய ஆர்டர் தொடங்கு",
  verified: "சரிபார்க்கப்பட்ட வணிகம்", notVerified: "இன்னும் சரிபார்த்த ஆவணங்கள் இல்லை. முதல் KYC க்குப் பிறகு சேமிக்கப்படும்.", verifiedOn: "சரிபார்த்த நாள்",
  reuseTitle: "சரிபார்த்த ஆவணங்களைப் பயன்படுத்தவும்", reuseBody: "உங்கள் வணிகத்தை முன்பே சரிபார்த்தோம். எதுவும் மாறவில்லை என்றால் எதையும் பதிவேற்ற வேண்டாம்.",
  unchanged: "எதுவும் மாறவில்லை: வணிகப் பெயர், முகவரி, GSTIN, அங்கீகரிக்கப்பட்ட கையொப்பமிடுபவர் அதே.", reuseButton: "சரிபார்த்த விவரங்களைப் பயன்படுத்து", somethingChanged: "ஏதாவது மாறியதா? கீழே புதிய ஆவணங்களைப் பதிவேற்றவும்.",
  reuseNotPossible: "புதிய ஆவணங்கள் தேவை:", signInToSkip: "முன்பு ஆர்டர் செய்தீர்களா? ஆவணங்களைத் தவிர்க்க உள்நுழையவும்.", current: "தற்போதைய", reusedKyc: "KYC மீண்டும் பயன்பாடு", back: "பின்செல்", finishKyc: "சரிபார்ப்பை முடி",
};
const BY: Partial<Record<LangCode, Partial<Record<AccountTextKey, string>>>> = { hi: HI, ta: TA };
/** Account and KYC-reuse text (web and mobile). */
export const accountText = (lang: LangCode, k: AccountTextKey) => BY[lang]?.[k] ?? EN[k];
