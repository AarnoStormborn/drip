package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// wordmark is the ASCII "DRIP" logo (pure ASCII, renders reliably everywhere).
const wordmark = `       __            __           
      /  |          /  |          
  ____$$ |  ______  $$/   ______  
 /    $$ | /      \ /  | /      \ 
/$$$$$$$ |/$$$$$$  |$$ |/$$$$$$  |
$$ |  $$ |$$ |  $$/ $$ |$$ |  $$ |
$$ \__$$ |$$ |      $$ |$$ |__$$ |
$$    $$ |$$ |      $$ |$$    $$/ 
 $$$$$$$/ $$/       $$/ $$$$$$$/  
                        $$ |      
                        $$ |      
                        $$/       `

var (
	bannerStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("213"))
	taglineStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
)

// minBannerWidth is the minimum terminal width for the full ASCII wordmark;
// below it the header falls back to a one-line title.
const minBannerWidth = 84

// renderHeader builds the top of every screen: the ASCII wordmark (or a
// one-line title on narrow terminals) with the tagline and tab row.
func (a *App) renderHeader() string {
	wide := a.w >= minBannerWidth

	var b strings.Builder
	if wide {
		b.WriteString(bannerStyle.Render(wordmark))
		b.WriteString("\n")
	}

	tagline := "Drip — subscription tracker"
	if wide {
		tagline = "drip · your subscriptions, at a glance"
	}
	tabs := a.renderTabs()

	// tagline left, tabs right (approximate right-alignment)
	pad := a.w - len(tagline) - lipgloss.Width(tabs)
	if pad < 2 {
		pad = 2
	}
	b.WriteString(fmt.Sprintf("%s%s%s", taglineStyle.Render(tagline), strings.Repeat(" ", pad), tabs))
	return b.String()
}
