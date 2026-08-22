// SPDX-License-Identifier: AGPL-3.0-or-later

package cmd

import "testing"

func TestUpdateDocumentsCommandRegistration(t *testing.T) {
	command, _, err := rootCmd.Find([]string{"update"})
	if err != nil {
		t.Fatal(err)
	}
	if command != updateDocumentsCmd {
		t.Fatalf("update command = %q, want %q", command.Name(), updateDocumentsCmd.Name())
	}
	for _, name := range []string{"user-id", "label", "title", "language", "dry", "yes"} {
		if command.Flags().Lookup(name) == nil {
			t.Errorf("update command is missing --%s", name)
		}
	}
	if got := command.Annotations[executionScopeAnnotation]; got != string(executionScopeRemote) {
		t.Fatalf("update execution scope = %q, want remote", got)
	}
}
