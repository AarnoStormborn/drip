// Package detect finds recurring payments in statement records and upserts
// them as subscriptions. It uses two signals:
//
//  1. UPI AutoPay marker — HDFC narrations that contain "UPI-AUTOPAY-" are
//     e-mandate debits by construction (the bank's recurring-payment rail),
//     so a single occurrence is enough to create a subscription.
//  2. Recurring heuristic — any merchant charged the same amount in >= 2
//     distinct periods (monthly/quarterly/yearly cadence) is a candidate.
//
// Descriptors are parsed from normalized text, e.g.
// "UPI-AUTOPAY-NETFLIX-NETFLIX.BD@AXISBANK-…-MANDATEEXECUTE" → merchant "netflix".
package detect

import (
	"strings"
)

// NormalizeDescriptor reduces a raw statement description to a lowercase
// alphanumeric key for grouping, e.g.
//
//	"UPI-AUTOPAY-NETFLIX-NETFLIX.BD@AXISBANK- UTIB0000100-747434051436-MANDATEEXECUTE HDFCBANK LIMITED"
//	  → "upiautopaynetflix"
func NormalizeDescriptor(desc string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(desc) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		}
	}
	return b.String()
}

// tokenize splits a descriptor into tokens on common separators.
func tokenize(desc string) []string {
	repl := strings.NewReplacer("-", " ", "/", " ", "_", " ", ".", " ", ",", " ", "@", " @ ")
	return strings.Fields(repl.Replace(desc))
}

// stripPSP removes an "@psp" suffix and anything after it.
func stripPSP(s string) string {
	if i := strings.Index(s, "@"); i >= 0 {
		return s[:i]
	}
	return s
}

// masked reports whether a token looks like a masked identifier (bank-masked
// UPI numbers start with a run of Xs), e.g. "XXXXXXXX0711" — these are
// transfers to masked numbers, not merchants.
func masked(s string) bool {
	lower := strings.ToLower(s)
	return strings.HasPrefix(lower, "xxxx")
}

// ExtractMerchant returns the merchant token of a statement description and
// whether the description looks like a merchant payment at all.
func ExtractMerchant(desc string) (string, bool) {
	toks := tokenize(desc)
	if len(toks) == 0 {
		return "", false
	}

	var merchant string
	switch strings.ToUpper(toks[0]) {
	case "UPI":
		// UPI-AUTOPAY-NETFLIX-... → "netflix"
		// UPI-REGALTRADERS-...@OKBIZAXIS-... → "regaltraders"
		if len(toks) >= 3 && strings.ToUpper(toks[1]) == "AUTOPAY" {
			merchant = stripPSP(toks[2])
		} else if len(toks) >= 2 {
			merchant = stripPSP(toks[1])
		}
	case "ACHD":
		// ACHD-ETMONEY-ETMONEYXXX... → "etmoney"
		if len(toks) >= 2 {
			merchant = stripPSP(toks[1])
		}
	case "CHQPAID":
		// CHQPAID-CTSS6-MUMB-LODHA DEVELOPERS → "LODHA DEVELOPERS"
		if len(toks) >= 4 {
			merchant = strings.Join(toks[3:], " ")
		}
	default:
		// POS / IMPS / NEFT / RTGS / SMS / bank charges — not merchants
		return "", false
	}

	if merchant == "" || masked(merchant) {
		return "", false
	}
	return merchant, true
}

// serviceNames maps normalized merchant keys to friendly service names.
var serviceNames = map[string]string{
	"netflix":         "Netflix",
	"spotify":         "Spotify",
	"spotifyindiallp": "Spotify",
	"spotifyab":       "Spotify",
	"apple":           "Apple",
	"applemedia":      "Apple Media",
	"applemusic":      "Apple Music",
	"appleone":        "Apple One",
	"appleservices":   "Apple",
	"wwwairtel":       "Airtel",
	"airtel":          "Airtel",
	"wispr":           "Wispr Flow",
	"wisprflow":       "Wispr Flow",
	"paytm":           "Paytm",
	"axio":            "Paytm",
	"axiopaytm":       "Paytm",
	"etmoney":         "ETMoney",
	"amazon":          "Amazon",
	"prime":           "Amazon Prime",
	"primevideo":      "Amazon Prime Video",
	"youtube":         "YouTube",
	"youtubepremium":  "YouTube Premium",
	"hotstar":         "Disney+ Hotstar",
	"disney":          "Disney+ Hotstar",
	"jiotv":           "JioTV",
	"jio":             "Jio",
	"zomato":          "Zomato",
	"swiggy":          "Swiggy",
	"sonyliv":         "SonyLIV",
	"canva":           "Canva",
	"notion":          "Notion",
	"figma":           "Figma",
	"openai":          "ChatGPT",
	"chatgpt":         "ChatGPT",
	"googleone":       "Google One",
	"icloud":          "iCloud",
	"audible":         "Audible",
	"kindle":          "Kindle",
	"crunchyroll":     "Crunchyroll",
	"linkedin":        "LinkedIn",
	"microsoft365":    "Microsoft 365",
	"github":          "GitHub",
}

// ServiceName turns a merchant token into a friendly display name.
func ServiceName(merchant string) string {
	key := NormalizeDescriptor(merchant)
	if name, ok := serviceNames[key]; ok {
		return name
	}
	return titleCase(merchant)
}

func titleCase(s string) string {
	words := strings.Fields(s)
	for i, w := range words {
		w = strings.ToLower(w)
		if len(w) > 0 {
			words[i] = strings.ToUpper(w[:1]) + w[1:]
		}
	}
	return strings.Join(words, " ")
}
