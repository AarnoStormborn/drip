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
// below it the header falls back to a one-line title. The wordmark is ~36
// chars wide, so this comfortably fits 80-column terminals.
const minBannerWidth = 50

// marginLeft indents the banner from the left edge so it doesn't hug the
// terminal border; the rest of the header stack stays flush.
const marginLeft = 2

// renderBanner draws the top of every screen: the ASCII wordmark, or a
// one-line title on narrow terminals.
func (a *App) renderBanner() string {
	if a.w >= minBannerWidth {
		return bannerStyle.PaddingLeft(marginLeft).Render(wordmark)
	}
	return taglineStyle.PaddingLeft(marginLeft).Render("Drip — subscription tracker")
}
