package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbletea"

	"drip/internal/model"
)

type formKind int

const (
	fText formKind = iota
	fChoice
)

type formField struct {
	label   string
	kind    formKind
	value   string
	choices []string
	input   textinput.Model
}

// editForm edits a subscription's core fields. Text fields use a bubbles
// textinput; choice fields cycle with ←/→. Enter moves focus. Lifecycle
// (cancel/save) is owned by the App: it checks Cancelled/Err after Update.
type editForm struct {
	fields    []*formField
	idx       int
	targetID  int64
	Err       string
	Cancelled bool
}

func newEditForm(s model.Subscription) *editForm {
	f := &editForm{targetID: s.ID}
	cycleChoices := []string{"weekly", "monthly", "quarterly", "half-yearly", "yearly", "once"}
	statusChoices := []string{"active", "paused", "cancelled"}
	railChoices := []string{"upi_autopay", "card_emandate", "app_store", "merchant_direct", "bank_si"}

	f.fields = []*formField{
		{label: "Service", kind: fText, value: s.Service},
		{label: "Amount (₹)", kind: fText, value: formatRupees(s.Amount)},
		{label: "Category", kind: fText, value: s.Category},
		{label: "Next due (YYYY-MM-DD)", kind: fText, value: s.NextPaymentDate},
		{label: "Cycle", kind: fChoice, value: s.Cycle, choices: cycleChoices},
		{label: "Status", kind: fChoice, value: s.Status, choices: statusChoices},
		{label: "Rail", kind: fChoice, value: s.Rail, choices: railChoices},
	}
	for _, fl := range f.fields {
		if fl.kind == fText {
			ti := textinput.New()
			ti.Placeholder = fl.label
			ti.SetValue(fl.value)
			ti.CharLimit = 80
			fl.input = ti
		}
	}
	f.fields[0].input.Focus()
	return f
}

func (f *editForm) current() *formField { return f.fields[f.idx] }

func (f *editForm) move(delta int) {
	prev := f.idx
	f.idx += delta
	if f.idx < 0 {
		f.idx = len(f.fields) - 1
	}
	if f.idx >= len(f.fields) {
		f.idx = 0
	}
	if f.current().kind == fText {
		f.current().input.Focus()
	}
	if f.fields[prev].kind == fText {
		f.fields[prev].input.Blur()
	}
}

func (f *editForm) cycle(delta int) {
	fl := f.current()
	if fl.kind != fChoice {
		return
	}
	idx := -1
	for i, c := range fl.choices {
		if c == fl.value {
			idx = i
			break
		}
	}
	if idx < 0 {
		idx = 0
	}
	idx = (idx + delta + len(fl.choices)) % len(fl.choices)
	fl.value = fl.choices[idx]
}

// Update handles navigation and typing. It does not consume esc/ctrl+s —
// the App interprets those (cancel/save lifecycle).
func (f *editForm) Update(msg tea.Msg) tea.Cmd {
	switch m := msg.(type) {
	case tea.KeyMsg:
		switch m.String() {
		case "up":
			f.move(-1)
			return nil
		case "down":
			f.move(1)
			return nil
		case "enter":
			if f.current().kind == fText {
				f.move(1)
			} else {
				f.cycle(1)
			}
			return nil
		case "left":
			f.cycle(-1)
			return nil
		case "right":
			f.cycle(1)
			return nil
		default:
			if fl := f.current(); fl.kind == fText {
				fl.input, _ = fl.input.Update(m)
				// bubbles returns the updated model + cmd; cursor blink is a
				// nice-to-have we intentionally drop for simplicity
			}
		}
	case tea.WindowSizeMsg:
		for _, fl := range f.fields {
			if fl.kind == fText {
				fl.input.Width = m.Width - 34
			}
		}
	}
	return nil
}

// Build validates the fields and returns the subscription to persist.
func (f *editForm) Build(s model.Subscription) (model.Subscription, error) {
	svc := strings.TrimSpace(f.fields[0].input.Value())
	if svc == "" {
		return s, fmt.Errorf("service cannot be empty")
	}
	amount, err := parseRupees(f.fields[1].input.Value())
	if err != nil {
		return s, err
	}
	next := strings.TrimSpace(f.fields[3].input.Value())
	if next != "" {
		if _, err := time.Parse("2006-01-02", next); err != nil {
			return s, fmt.Errorf("next due must be YYYY-MM-DD (or empty)")
		}
	}

	s.Service = svc
	s.Amount = amount
	s.Category = strings.TrimSpace(f.fields[2].input.Value())
	s.NextPaymentDate = next
	s.Cycle = f.fields[4].value
	s.Status = f.fields[5].value
	s.Rail = f.fields[6].value
	return s, nil
}

// View renders the form.
func (f *editForm) View() string {
	var b strings.Builder
	b.WriteString(headerStyle.Render("EDIT SUBSCRIPTION"))
	b.WriteString("\n\n")
	for i, fl := range f.fields {
		marker := "  "
		if i == f.idx {
			marker = "▸ "
		}
		label := fmt.Sprintf("%-22s", fl.label)
		var value string
		switch fl.kind {
		case fText:
			value = fl.input.View()
		case fChoice:
			value = fl.value
			if i == f.idx {
				value = accentStyle.Render(fl.value + "  ←/→ cycle")
			}
		}
		b.WriteString(marker + infoLabel.Render(label) + value + "\n")
	}
	if f.Err != "" {
		b.WriteString("\n" + errorStyle.Render("⚠ "+f.Err) + "\n")
	}
	b.WriteString("\n" + hintStyle.Render("↑/↓ move · ←/→ cycle · enter next · ctrl+s save · esc cancel"))
	return b.String()
}
