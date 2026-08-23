// SPDX-FileContributor: 4evy <git@evy.pink>
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/asciimoo/hister/config"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var createConfigCmd = &cobra.Command{
	Use:   "create-config [FILENAME]",
	Short: "Create default configuration file",
	Long:  "Write the default configuration to FILENAME, or print it to standard output when no filename is given.",
	Args:  cobra.MaximumNArgs(1),
	Run: func(_ *cobra.Command, args []string) {
		dcfg := config.CreateDefaultConfig()
		cb, err := yaml.Marshal(dcfg)
		if err != nil {
			panic(err)
		}
		if len(args) > 0 {
			fname := args[0]
			if _, err := os.Stat(fname); err == nil {
				exit(1, fmt.Sprintf(`File "%s" already exists`, fname))
			}
			if err := writeDefaultConfigFile(fname, cb); err != nil {
				exit(1, `Failed to create config file: `+err.Error())
			}
			cliPrintln(cliSuccessStyle.Render("✓") + " Config file created: " + cliInfoStyle.Render(fname))
		} else {
			fmt.Print(string(cb))
		}
	},
}

func writeDefaultConfigFile(filename string, content []byte) error {
	if err := os.MkdirAll(filepath.Dir(filename), 0o700); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}
	if err := os.WriteFile(filename, content, 0o600); err != nil {
		return err
	}
	return nil
}
