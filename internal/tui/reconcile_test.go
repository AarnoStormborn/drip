package tui

import (
	"testing"

	"drip/internal/model"
)

func sub(id int64, service string, amount int64, rail, status string) model.Subscription {
	return model.Subscription{ID: id, Service: service, Amount: amount, Rail: rail, Status: status}
}

func mand(id int64, service string, amount int64, status string) model.Mandate {
	return model.Mandate{ID: id, Service: service, Amount: amount, Status: status}
}

func TestComputeReconcile(t *testing.T) {
	subs := []model.Subscription{
		sub(1, "Netflix", 64900, "upi_autopay", "active"),
		sub(2, "Airtel", 52982, "upi_autopay", "active"),
		{ID: 3, Service: "Paytm", Amount: 329766, Rail: "upi_autopay", Status: "active",
			Descriptors: `["upiautopayaxiopaytmaxiocfpaytmyesb0ptmupi615600465406"]`},
		sub(4, "Spotify", 14900, "card_emandate", "active"), // card rail — exempt from "not in mandates"
		sub(5, "Apple Media", 21900, "upi_autopay", "cancelled"),
	}
	mandates := []model.Mandate{
		mand(1, "Netflix", 64900, "active"),      // match (exact)
		mand(2, "AXIO", 329766, "active"),        // match Paytm via descriptor containment
		mand(3, "New Service", 99900, "active"),  // no match → missing
		mand(4, "Airtel", 50000, "active"),       // amount mismatch
		mand(5, "Old Thing", 10000, "cancelled"), // inactive — ignored
	}

	res := computeReconcile(subs, mandates)

	if len(res.MissingSubs) != 1 || res.MissingSubs[0].Service != "New Service" {
		t.Errorf("MissingSubs = %+v, want [New Service]", res.MissingSubs)
	}
	// every active upi_autopay sub has a mandate: Netflix ✓, Airtel ✓ (amount
	// mismatch), Paytm ✓ (via descriptor) — so nothing is "not in mandates"
	if len(res.NotInMandates) != 0 {
		t.Errorf("NotInMandates = %+v, want []", res.NotInMandates)
	}
	if len(res.AmountMismatch) != 1 || res.AmountMismatch[0].Mandate.Service != "Airtel" {
		t.Errorf("AmountMismatch = %+v, want [Airtel]", res.AmountMismatch)
	}
	// matched: Netflix, AXIO→Paytm, Airtel
	if len(res.Matched) != 3 {
		t.Errorf("Matched = %+v, want 3", res.Matched)
	}
}

func TestMatchMandateDescriptorFallback(t *testing.T) {
	// sub "Paytm" has descriptor key containing "axio" — mandate "AXIO" matches
	subs := []model.Subscription{
		{ID: 1, Service: "Paytm", Amount: 329766, Rail: "upi_autopay", Status: "active",
			Descriptors: `["upiautopayaxiopaytmaxiocfpaytmyesb0ptmupi615600465406"]`},
	}
	m := model.Mandate{Service: "AXIO", Amount: 329766, Status: "active"}
	got, ok := matchMandate(m, subs)
	if !ok || got.ID != 1 {
		t.Errorf("matchMandate(AXIO) = %+v, %v; want sub 1", got, ok)
	}

	// no match for unrelated name
	_, ok = matchMandate(model.Mandate{Service: "Wispr Flow", Status: "active"}, subs)
	if ok {
		t.Error("unrelated mandate should not match")
	}
}

func TestMandateFormBuild(t *testing.T) {
	f := newMandateForm()
	f.fields[0].input.SetValue("Notion")
	f.fields[1].input.SetValue("349")
	f.fields[2].input.SetValue("2026-09-01")
	m, err := f.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if m.Service != "Notion" || m.Amount != 34900 || m.Cycle != "monthly" ||
		m.NextDebitDate != "2026-09-01" || m.Status != "active" || m.UPIApp != "gpay" {
		t.Errorf("mandate = %+v", m)
	}

	// empty service → error
	f.fields[0].input.SetValue("")
	if _, err := f.Build(); err == nil {
		t.Error("empty service should error")
	}
}
