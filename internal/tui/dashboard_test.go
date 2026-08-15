package tui

import (
	"testing"
	"time"

	"github.com/AarnoStormborn/drip/internal/model"
)

func TestBuildDashboardOverdueAndDueSoon(t *testing.T) {
	today := time.Now().Format("2006-01-02")
	week := time.Now().AddDate(0, 0, 7).Format("2006-01-02")
	far := time.Now().AddDate(0, 0, 30).Format("2006-01-02")
	past := time.Now().AddDate(0, 0, -3).Format("2006-01-02")

	subs := []model.Subscription{
		{Service: "OverdueSub", Amount: 10000, Cycle: "monthly", Status: "active", NextPaymentDate: past},
		{Service: "DueSoonSub", Amount: 20000, Cycle: "monthly", Status: "active", NextPaymentDate: week},
		{Service: "DueToday", Amount: 5000, Cycle: "monthly", Status: "active", NextPaymentDate: today},
		{Service: "LaterSub", Amount: 30000, Cycle: "yearly", Status: "active", NextPaymentDate: far},
	}

	d := buildDashboard(subs)
	if d.ActiveCount != 4 {
		t.Errorf("ActiveCount = %d, want 4", d.ActiveCount)
	}
	if len(d.Overdue) != 1 || d.Overdue[0].Service != "OverdueSub" {
		t.Errorf("Overdue = %+v, want [OverdueSub]", d.Overdue)
	}
	if len(d.DueSoon) != 2 {
		t.Errorf("DueSoon = %+v, want 2 (DueToday + DueSoonSub)", d.DueSoon)
	}
	// monthly spend in paise: 10000 + 20000 + 5000 + (30000/12) = 37500
	if d.MonthlySpend != 37500 {
		t.Errorf("MonthlySpend = %d, want 37500", d.MonthlySpend)
	}
}
