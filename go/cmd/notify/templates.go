package main

import (
	"sort"
	"strings"
)

// templates[key][language code] with {param} placeholders. English, Hindi and Tamil are written
// out (the plan's minimum language set); other languages fall back to English, recorded on the
// sent message as language "en".
var templates = map[string]map[string]string{
	"order.created": {
		"en": "Thank you! Your order {order} is confirmed. Next: finish KYC so we can schedule installation.",
		"hi": "धन्यवाद! आपका ऑर्डर {order} पक्का हो गया है। अगला कदम: KYC पूरा करें ताकि हम इंस्टॉलेशन तय कर सकें।",
		"ta": "நன்றி! உங்கள் ஆர்டர் {order} உறுதியானது. அடுத்து: நிறுவலைத் திட்டமிட KYC-ஐ முடிக்கவும்.",
	},
	"order.status.kyc_pending": {
		"en": "Order {order}: please upload your KYC documents to continue.",
		"hi": "ऑर्डर {order}: आगे बढ़ने के लिए कृपया अपने KYC दस्तावेज़ अपलोड करें।",
		"ta": "ஆர்டர் {order}: தொடர உங்கள் KYC ஆவணங்களைப் பதிவேற்றவும்.",
	},
	"order.status.kyc_passed": {
		"en": "Order {order}: KYC verified. Expected activation by {date}.",
		"hi": "ऑर्डर {order}: KYC सत्यापित हो गया। {date} तक चालू होने की उम्मीद है।",
		"ta": "ஆர்டர் {order}: KYC சரிபார்க்கப்பட்டது. {date}க்குள் செயல்படுத்தப்படும் என எதிர்பார்க்கிறோம்.",
	},
	"order.status.install_scheduled": {
		"en": "Order {order}: installation is scheduled. Expected activation by {date}.",
		"hi": "ऑर्डर {order}: इंस्टॉलेशन तय हो गया है। {date} तक चालू होने की उम्मीद है।",
		"ta": "ஆர்டர் {order}: நிறுவல் திட்டமிடப்பட்டது. {date}க்குள் செயல்படுத்தப்படும்.",
	},
	"order.status.installing": {
		"en": "Order {order}: our engineer is installing your services today.",
		"hi": "ऑर्डर {order}: हमारे इंजीनियर आज आपकी सेवाएँ लगा रहे हैं।",
		"ta": "ஆர்டர் {order}: எங்கள் பொறியாளர் இன்று உங்கள் சேவைகளை நிறுவுகிறார்.",
	},
	"order.status.activated": {
		"en": "Order {order}: your services are live. Welcome aboard!",
		"hi": "ऑर्डर {order}: आपकी सेवाएँ चालू हो गई हैं। स्वागत है!",
		"ta": "ஆர்டர் {order}: உங்கள் சேவைகள் இயங்கத் தொடங்கின. வரவேற்கிறோம்!",
	},
	"order.status.delayed": {
		"en": "Order {order} is delayed ({reason}). We will update you; no action needed unless we ask.",
		"hi": "ऑर्डर {order} में देरी है ({reason})। हम आपको बताते रहेंगे; जब तक हम न कहें, कुछ करने की ज़रूरत नहीं।",
		"ta": "ஆர்டர் {order} தாமதமாகிறது ({reason}). நாங்கள் தகவல் தருவோம்; நாங்கள் கேட்காதவரை எதுவும் செய்ய வேண்டாம்.",
	},
	"order.status.failed": {
		"en": "Order {order} could not be completed ({reason}). Our team will call you.",
		"hi": "ऑर्डर {order} पूरा नहीं हो सका ({reason})। हमारी टीम आपको कॉल करेगी।",
		"ta": "ஆர்டர் {order} முடிக்க இயலவில்லை ({reason}). எங்கள் குழு உங்களை அழைக்கும்.",
	},
	"order.fallback_offer": {
		"en": "Order {order}: fibre is not available at your address yet. We can install 5G AirFiber instead, usually much faster. Reply YES to switch.",
		"hi": "ऑर्डर {order}: आपके पते पर अभी फ़ाइबर उपलब्ध नहीं है। हम इसकी जगह 5G AirFiber लगा सकते हैं, जो आमतौर पर जल्दी लगता है। बदलने के लिए YES लिखें।",
		"ta": "ஆர்டர் {order}: உங்கள் முகவரியில் இன்னும் ஃபைபர் இல்லை. அதற்குப் பதில் 5G AirFiber நிறுவலாம், பொதுவாக விரைவாக முடியும். மாற்ற YES என பதிலளிக்கவும்.",
	},
	"kyc.resume": {
		"en": "You are almost done! Continue your KYC where you left off: {link}",
		"hi": "बस थोड़ा सा बाकी है! अपना KYC वहीं से जारी रखें: {link}",
		"ta": "கிட்டத்தட்ட முடிந்துவிட்டது! உங்கள் KYC-ஐ விட்ட இடத்திலிருந்து தொடருங்கள்: {link}",
	},
}

// languageCodes maps profile language names (contracts/profile.schema.json) to codes.
var languageCodes = map[string]string{
	"English": "en", "Hindi": "hi", "Bengali": "bn", "Telugu": "te", "Marathi": "mr", "Tamil": "ta",
	"Gujarati": "gu", "Kannada": "kn", "Malayalam": "ml", "Punjabi": "pa",
}

// render fills a template in the requested language, falling back to English. ok is false for
// an unknown template key.
func render(key, lang string, params map[string]string) (body, usedLang string, ok bool) {
	t, ok := templates[key]
	if !ok {
		return "", "", false
	}
	usedLang = lang
	text, has := t[lang]
	if !has {
		text, usedLang = t["en"], "en"
	}
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		text = strings.ReplaceAll(text, "{"+k+"}", params[k])
	}
	return text, usedLang, true
}
