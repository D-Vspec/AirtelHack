import type { BusinessProfile, LanguageName } from "@udyam/shared";

export interface EvalScenario {
  id: string;
  title: string;
  language: LanguageName;
  romanised?: boolean;
  /** Ground truth for the eligibility check (what the owner really is, not what the LLM extracted). */
  truth: BusinessProfile;
  turns: string[];
  expect?: {
    noQuote?: boolean;          // never quotes (out of scope only)
    exclude?: string[];         // must never appear in any quote
    include?: string[];         // must appear in the final quote
    onlyProducts?: string[];    // final quote contains nothing else
    cheaper?: boolean;          // last quote cheaper than the first
    accepted?: boolean;         // final quote accepted
    minInstallDays?: number;
    noQuoteBeforeTurn?: number; // first quote not before this turn (out-of-scope opener)
  };
}

const t = true, f = false;
const micro = "Micro retail & services", prof = "Professional offices", multi = "Small multi-site businesses", mfg = "Small manufacturers";

/** 16 scripted scenarios (docs/scenarios.md 1-9 plus guardrails). */
export const SCRIPTED: EvalScenario[] = [
  { id: "S01", title: "Kirana without GSTIN wants WhatsApp/UPI", language: "English",
    truth: { segment: micro, city: "Lucknow", cityTier: 2, employeeCount: 2, numSites: 1, hasGstin: f },
    turns: ["I run a kirana store in Lucknow. Just me and my brother, one shop. No GST registration. I want UPI billing and WhatsApp for orders."],
    expect: { exclude: ["P15", "P14"] } },
  { id: "S02", title: "Dental clinic, Tier 1, 6 staff", language: "English",
    truth: { segment: prof, city: "Chennai", cityTier: 1, employeeCount: 6, numSites: 1, hasGstin: t },
    turns: ["Hello, I have a dental clinic in Chennai with 6 staff, one location. We are GST registered.", "What about backup if the internet goes down?"],
    expect: { include: ["P16"] } },
  { id: "S03", title: "CA firm, 8 staff, asks for managed firewall", language: "English",
    truth: { segment: prof, city: "Mumbai", cityTier: 1, employeeCount: 8, numSites: 1, hasGstin: t },
    turns: ["We are a CA firm in Mumbai, 8 employees, single office, GST yes. We need a managed firewall for client data."],
    expect: { exclude: ["P21"] } },
  { id: "S04", title: "Tier 2 office asks for Tier 1 only fibre", language: "English",
    truth: { segment: prof, city: "Indore", cityTier: 2, employeeCount: 12, numSites: 1, hasGstin: t },
    turns: ["Architecture studio in Indore, 12 people, one office, GST registered. I want the 1 Gbps fibre."],
    expect: { exclude: ["P03"] } },
  { id: "S05", title: "Restaurant chain, 5 sites, Tier 1", language: "English",
    truth: { segment: multi, city: "Bengaluru", cityTier: 1, employeeCount: 40, numSites: 5, hasGstin: t },
    turns: ["I own a restaurant chain in Bengaluru with 5 outlets and about 40 staff. GST registered. I want CCTV at every outlet and one bill."],
    expect: { include: ["P23"] } },
  { id: "S06", title: "Manufacturer, 40 staff, honest install time", language: "English",
    truth: { segment: mfg, city: "Nashik", cityTier: 2, employeeCount: 40, numSites: 1, hasGstin: t },
    turns: ["Auto parts unit in Nashik, 40 workers, one factory, GSTIN yes. Need reliable internet and machine monitoring."],
    expect: { minInstallDays: 21 } },
  { id: "S07", title: "CCTV without internet", language: "English",
    truth: { segment: micro, city: "Jaipur", cityTier: 2, employeeCount: 3, numSites: 1, hasGstin: f },
    turns: ["Boutique in Jaipur, 3 staff, one shop, no GST. I only want 2 CCTV cameras, I don't need internet."],
    expect: { include: ["P23"] } },
  { id: "S08", title: "Only wants a mobile plan", language: "English",
    truth: { segment: micro, city: "Patna", cityTier: 2, employeeCount: 1, numSites: 1, hasGstin: f },
    turns: ["I just need a business mobile plan, one SIM. Nothing else please."],
    expect: { onlyProducts: ["P07", "P08"] } },
  { id: "S09", title: "Too expensive -> cheaper tier with total cost", language: "English",
    truth: { segment: prof, city: "Delhi", cityTier: 1, employeeCount: 5, numSites: 1, hasGstin: t },
    turns: ["Coaching centre in Delhi, 5 staff, one centre, GST registered.", "That is too expensive for me."],
    expect: { cheaper: true } },
  { id: "S10", title: "Unknown GSTIN", language: "English",
    truth: { segment: prof, city: "Pune", cityTier: 1, employeeCount: 10, numSites: 1, hasGstin: f },
    turns: ["Law office in Pune, 10 staff, one office. Our GSTIN is 27ABCDE1234F1Z5. We want cloud telephony."],
    expect: { exclude: ["P12", "P13", "P14", "P15", "P21", "P03"] } },
  { id: "S11", title: "Cancelled GSTIN (K01)", language: "English",
    truth: { segment: prof, city: "Visakhapatnam", cityTier: 2, employeeCount: 12, numSites: 1, hasGstin: f },
    turns: ["Travel agency in Visakhapatnam, 12 staff, one office. GSTIN 37IHKXV5167L1ZS. Need internet and a toll-free number."],
    expect: { exclude: ["P13", "P12", "P21"] } },
  { id: "S12", title: "Out-of-scope opener, then normal", language: "English",
    truth: { segment: micro, city: "Surat", cityTier: 2, employeeCount: 4, numSites: 1, hasGstin: t },
    turns: ["Can you help me file my income tax return?", "OK. I have a sweet shop in Surat, 4 staff, one shop, GST registered."],
    expect: { noQuoteBeforeTurn: 2 } },
  { id: "S13", title: "Sparse answers over several turns", language: "English",
    truth: { segment: prof, city: "Hyderabad", cityTier: 1, employeeCount: 7, numSites: 1, hasGstin: t },
    turns: ["hi", "diagnostic lab", "Hyderabad", "7 people, one lab", "yes we have GST"] },
  { id: "S14", title: "Valid GSTIN unlocks GST products", language: "English",
    truth: { segment: prof, city: "Kochi", cityTier: 2, employeeCount: 20, numSites: 1, hasGstin: t },
    turns: ["Balaji Dental Care LLP in Kochi, 20 staff, one clinic, GSTIN 32TTNXB7384T1ZW. We want cloud telephony with IVR."],
    expect: { include: ["P12"] } },
  { id: "S15", title: "Recommend then accept", language: "English",
    truth: { segment: micro, city: "Bhopal", cityTier: 2, employeeCount: 2, numSites: 1, hasGstin: f },
    turns: ["Pharmacy in Bhopal, 2 staff, one shop, no GST yet.", "Yes, go ahead with this."],
    expect: { accepted: true } },
  { id: "S16", title: "2-site business asks for SD-WAN", language: "English",
    truth: { segment: multi, city: "Ahmedabad", cityTier: 1, employeeCount: 25, numSites: 2, hasGstin: t },
    turns: ["Gym chain in Ahmedabad with 2 branches and 25 staff, GST registered. We want SD-WAN to manage both."],
    expect: { exclude: ["P10"] } },
];

/** 11 multilingual conversations: every language in the case data plus Hinglish and Tanglish. */
export const MULTILINGUAL: EvalScenario[] = [
  { id: "M01", title: "Hindi kirana, no GST", language: "Hindi",
    truth: { segment: micro, city: "Lucknow", cityTier: 2, employeeCount: 2, numSites: 1, hasGstin: f },
    turns: ["नमस्ते, लखनऊ में मेरी किराने की एक दुकान है। हम दो लोग हैं। GST रजिस्ट्रेशन नहीं है। UPI से पेमेंट लेना है।"],
    expect: { exclude: ["P15", "P14"] } },
  { id: "M02", title: "Hinglish kirana", language: "Hindi", romanised: true,
    truth: { segment: micro, city: "Patna", cityTier: 2, employeeCount: 3, numSites: 1, hasGstin: f },
    turns: ["bhai meri Patna mein ek kirana dukaan hai, 3 log kaam karte hain, GST nahi hai. internet aur billing chahiye"] },
  { id: "M03", title: "Tamil clinic", language: "Tamil",
    truth: { segment: prof, city: "Chennai", cityTier: 1, employeeCount: 6, numSites: 1, hasGstin: t },
    turns: ["வணக்கம், சென்னையில் எனக்கு ஒரு பல் மருத்துவமனை உள்ளது. 6 ஊழியர்கள், ஒரே கிளை. GST பதிவு உள்ளது."] },
  { id: "M04", title: "Tanglish salon", language: "Tamil", romanised: true,
    truth: { segment: micro, city: "Coimbatore", cityTier: 2, employeeCount: 4, numSites: 1, hasGstin: t },
    turns: ["Vanakkam, Coimbatore la oru salon vachirukken, 4 staff, oru kadai thaan. GST irukku. Internet and billing venum."] },
  { id: "M05", title: "Bengali salon", language: "Bengali",
    truth: { segment: micro, city: "Kolkata", cityTier: 1, employeeCount: 3, numSites: 1, hasGstin: f },
    turns: ["নমস্কার, কলকাতায় আমার একটা সেলুন আছে। ৩ জন কর্মী, একটাই দোকান। GST নেই।"] },
  { id: "M06", title: "Telugu coaching centre", language: "Telugu",
    truth: { segment: prof, city: "Hyderabad", cityTier: 1, employeeCount: 8, numSites: 1, hasGstin: t },
    turns: ["నమస్తే, హైదరాబాద్‌లో నాకు ఒక కోచింగ్ సెంటర్ ఉంది. 8 మంది సిబ్బంది, ఒకే బ్రాంచ్. GST రిజిస్ట్రేషన్ ఉంది."] },
  { id: "M07", title: "Marathi bakery", language: "Marathi",
    truth: { segment: micro, city: "Pune", cityTier: 1, employeeCount: 4, numSites: 1, hasGstin: t },
    turns: ["नमस्कार, पुण्यात माझी एक बेकरी आहे. ४ कर्मचारी, एकच दुकान. GST नोंदणी आहे. बिलिंग आणि इंटरनेट हवे आहे."] },
  { id: "M08", title: "Gujarati textile unit", language: "Gujarati",
    truth: { segment: mfg, city: "Surat", cityTier: 2, employeeCount: 30, numSites: 1, hasGstin: t },
    turns: ["નમસ્તે, સુરતમાં મારું એક ટેક્સટાઇલ યુનિટ છે. 30 કામદારો, એક ફેક્ટરી. GST રજીસ્ટ્રેશન છે. મશીન મોનિટરિંગ જોઈએ છે."] },
  { id: "M09", title: "Kannada architecture studio", language: "Kannada",
    truth: { segment: prof, city: "Bengaluru", cityTier: 1, employeeCount: 9, numSites: 1, hasGstin: t },
    turns: ["ನಮಸ್ಕಾರ, ಬೆಂಗಳೂರಿನಲ್ಲಿ ನನ್ನದು ಒಂದು ಆರ್ಕಿಟೆಕ್ಚರ್ ಸ್ಟುಡಿಯೋ ಇದೆ. 9 ಸಿಬ್ಬಂದಿ, ಒಂದೇ ಕಚೇರಿ. GST ನೋಂದಣಿ ಇದೆ."] },
  { id: "M10", title: "Punjabi sweet shop", language: "Punjabi",
    truth: { segment: micro, city: "Ludhiana", cityTier: 2, employeeCount: 5, numSites: 1, hasGstin: f },
    turns: ["ਸਤ ਸ੍ਰੀ ਅਕਾਲ, ਲੁਧਿਆਣਾ ਵਿੱਚ ਮੇਰੀ ਮਿਠਾਈ ਦੀ ਦੁਕਾਨ ਹੈ। 5 ਕਰਮਚਾਰੀ, ਇੱਕ ਦੁਕਾਨ। GST ਨਹੀਂ ਹੈ।"],
    expect: { exclude: ["P15", "P14"] } },
  { id: "M11", title: "Malayalam dental clinic", language: "Malayalam",
    truth: { segment: prof, city: "Kochi", cityTier: 2, employeeCount: 6, numSites: 1, hasGstin: t },
    turns: ["നമസ്കാരം, കൊച്ചിയിൽ എനിക്ക് ഒരു ഡെന്റൽ ക്ലിനിക് ഉണ്ട്. 6 ജീവനക്കാർ, ഒരു ക്ലിനിക്. GST രജിസ്ട്രേഷൻ ഉണ്ട്."] },
];
