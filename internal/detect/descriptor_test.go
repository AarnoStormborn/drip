package detect

import "testing"

func TestExtractMerchant(t *testing.T) {
	cases := []struct {
		desc   string
		want   string
		wantOK bool
		label  string
	}{
		{"UPI-AUTOPAY-NETFLIX-NETFLIX.BD@AXISBANK- UTIB0000100-747434051436-MANDATEEXECUTE", "NETFLIX", true, "Netflix"},
		{"UPI-AUTOPAY-SPOTIFYINDIALLP-SPOTIFY.BD SI@ICICI-ICIC0DC0099-109882080483-MANDAT EREQUEST", "SPOTIFYINDIALLP", true, "Spotify"},
		{"UPI-AUTOPAY-WWWAIRTEL IN-AIRTELAUTOPAY. PAYU@HDFCBANK-HDFC0MERUPI-103258019636-U HDFCBANK LIMITED", "WWWAIRTEL", true, "Airtel"},
		{"UPI-AUTOPAY-AIRTEL-AIRTEL4-PAYU@ICICI-ICIC0DC0099-110263616573-UPI MANDATE", "AIRTEL", true, "Airtel"},
		{"UPI-AUTOPAY-WISPR FLOW-WISPRFLOW.CFP@CAS HFREENSDLPB-NSPB0000011-361746900128-MAN DATEREQUEST", "WISPR", true, "Wispr Flow"},
		{"UPI-AUTOPAY-AXIO-PAYTM-AXIOCF@PAYTM-YESB 0PTMUPI-615600465406-OID24DA475AD7524F6", "AXIO", true, "Paytm"},
		{"UPI-AUTOPAY-SONYLIV-SONYLIVHYP@YESPAY-YESB0YESUPI-620689270973-RECURRINGTXN", "SONYLIV", true, "SonyLIV"},
		{"UPI-AUTOPAY-APPLEMEDIA SERVICES-APPLESE RVICES.BDSI@HDFCBANK-HDFC0MERUPI-1032636 21524-EXECUTIONTEST", "APPLEMEDIA", true, "Apple Media"},
		{"UPI-AUTOPAY-ANOMALY-ANOMALY.CBF@AXISBANK-UTIB0001920-564216551856-MANDATEEXECUTE", "ANOMALY", true, "Anomaly"},
		{"UPI-REGALTRADERS-9890133181@OKBIZAXIS-U TIB0000553-122505801983-UPI", "REGALTRADERS", true, "Regaltraders"},
		{"ACHD-ETMONEY-ETMONEYXXXSKCDZGAQ2612318", "ETMONEY", true, "ETMoney"},
		{"CHQPAID-CTSS6-MUMB-LODHA DEVELOPERS", "LODHA DEVELOPERS", true, "Lodha Developers"},
		{"UPI-XXXXXXXX0711-ICIC0001045-122581884601-UPI", "", false, ""},
		{"POS223487XXXXXX1028339209", "", false, ""},
		{"SMS-EPR2714044 EPR2714044496761", "", false, ""},
		{"IMPS-612800974672-NIUMPTELTD-IDFB-XXXX", "", false, ""},
		{"INTERESTPAIDTILL 30-JUN-2026", "", false, ""},
		{"JANMAR26INSTAALERTCHG16", "", false, ""},
	}
	for _, c := range cases {
		got, ok := ExtractMerchant(c.desc)
		if ok != c.wantOK || (ok && got != c.want) {
			t.Errorf("ExtractMerchant(%q) = (%q, %v), want (%q, %v)", c.desc, got, ok, c.want, c.wantOK)
		}
		if c.wantOK && ServiceName(got) != c.label {
			t.Errorf("ServiceName(%q) = %q, want %q", got, ServiceName(got), c.label)
		}
	}
}

func TestNormalizeDescriptor(t *testing.T) {
	got := NormalizeDescriptor("UPI-AUTOPAY-NETFLIX-NETFLIX.BD@AXISBANK- UTIB0000100-747434051436-MANDATEEXECUTE")
	want := "upiautopaynetflixnetflixbdaxisbankutib0000100747434051436mandateexecute"
	if got != want {
		t.Errorf("NormalizeDescriptor = %q, want %q", got, want)
	}
}
