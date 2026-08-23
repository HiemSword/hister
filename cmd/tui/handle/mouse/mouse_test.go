// SPDX-FileContributor: 4evy <git@evy.pink>
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package mouse

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestNewEventPreservesTypedMouseMessages(t *testing.T) {
	tests := []struct {
		name       string
		msg        tea.MouseMsg
		wantAction action
		wantButton tea.MouseButton
	}{
		{name: "click", msg: tea.MouseClickMsg{X: 3, Y: 4, Button: tea.MouseLeft}, wantAction: actionClick, wantButton: tea.MouseLeft},
		{name: "release", msg: tea.MouseReleaseMsg{X: 3, Y: 4, Button: tea.MouseLeft}, wantAction: actionRelease, wantButton: tea.MouseLeft},
		{name: "motion", msg: tea.MouseMotionMsg{X: 3, Y: 4, Button: tea.MouseLeft}, wantAction: actionMotion, wantButton: tea.MouseLeft},
		{name: "wheel", msg: tea.MouseWheelMsg{X: 3, Y: 4, Button: tea.MouseWheelDown}, wantAction: actionWheel, wantButton: tea.MouseWheelDown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := newEvent(tt.msg)
			if event.X != 3 || event.Y != 4 || event.Button != tt.wantButton || event.Action != tt.wantAction {
				t.Fatalf("newEvent() = %#v, want position (3,4), button %v, action %v", event, tt.wantButton, tt.wantAction)
			}
		})
	}
}
