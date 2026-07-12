package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
)

func (m Model) View() tea.View {
	var b strings.Builder
	b.WriteString(titleStyle.Render("P2P File Transfer Setup"))
	b.WriteString("\n\n")

	senderText, receiverText := " Sender ", " Receiver "
	if m.role == RoleSender {
		senderText = activeStyle.Render(senderText)
		receiverText = inactiveStyle.Render(receiverText)
	} else {
		senderText = inactiveStyle.Render(senderText)
		receiverText = activeStyle.Render(receiverText)
	}

	if m.focusIndex == FocusToggle {
		fmt.Fprintf(&b, "> [ %s | %s ]\n\n", senderText, receiverText)
	} else {
		fmt.Fprintf(&b, "  [ %s | %s ]\n\n", senderText, receiverText)
	}

	if m.focusIndex == FocusPath {
		b.WriteString(activeStyle.Render("> "))
		b.WriteString(m.pathInput.View())
		b.WriteString("\n\n")
	} else {
		b.WriteString("  ")
		b.WriteString(m.pathInput.View())
		b.WriteString("\n\n")
	}

	if m.role == RoleSender {
		fmt.Fprintf(&b, "  Your Unique ID: %s\n\n", activeStyle.Render(m.totpSecret))
	} else {
		if m.focusIndex == FocusTOTP {
			b.WriteString(activeStyle.Render("> "))
			b.WriteString(m.totpInput.View())
			b.WriteString("\n\n")
		} else {
			b.WriteString("  ")
			b.WriteString(m.totpInput.View())
			b.WriteString("\n\n")
		}
	}

	b.WriteString(inactiveStyle.Render("--- Connection Logs ---"))
	b.WriteString("\n")
	for _, l := range m.logs {
		b.WriteString(logStyle.Render(l))
		b.WriteString("\n")
	}
	b.WriteString("\n")

	b.WriteString(m.help.View(keys))

	// Return a declarative view
	return tea.NewView(b.String())
}
