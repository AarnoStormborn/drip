package tui

import (
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

// renderBanner draws the top of every screen: the ASCII wordmark, or a
// one-line title on narrow terminals.
func (a *App) renderBanner() string {
	if a.w >= minBannerWidth {
		return bannerStyle.Render(wordmark)
	}
	return taglineStyle.Render("Drip — subscription tracker")
}
