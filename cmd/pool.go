// Copyright (c) 2026 Jordi Gómez Hidalgo
// 
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// http://www.eclipse.org/legal/epl-2.0.
// 
// SPDX-License-Identifier: EPL-2.0

package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/gnugomez/scribe/store"
	"github.com/spf13/cobra"
)

func newPoolCmd(p store.EditPool, storePath string) *cobra.Command {
	var debug bool

	cmd := &cobra.Command{
		Use:          "pool",
		Short:        "Show the current pool contents",
		Long:         `pool prints all LLM tool usage events accumulated since the last amend.`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()

			if storePath == "" {
				noRepo()
			}

			entries, err := p.Peek()
			if err != nil {
				return fmt.Errorf("reading pool: %w", err)
			}
			if len(entries) == 0 {
				fmt.Fprintln(out, "pool is empty")
				return nil
			}

			for _, e := range entries {
				fmt.Fprintf(out, "%s  %s:%s\n",
					e.Timestamp.Format("2006-01-02T15:04:05Z"),
					e.Vendor, e.Model,
				)
				if debug {
					if e.ModelSource != "" {
						fmt.Fprintf(out, "  model source: %s\n", e.ModelSource)
					}
					fmt.Fprint(out, formatPayloadDebug(e.Payload))
				}
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&debug, "debug", false, "Show parsed payload details for each entry")
	return cmd
}

func formatPayloadDebug(raw string) string {
	if raw == "" {
		return "  payload: <empty>\n"
	}

	var compact bytes.Buffer
	if err := json.Compact(&compact, []byte(raw)); err != nil {
		return fmt.Sprintf("  payload (raw): %s\n", raw)
	}

	var pretty bytes.Buffer
	if err := json.Indent(&pretty, compact.Bytes(), "  ", "  "); err != nil {
		return fmt.Sprintf("  payload (raw): %s\n", raw)
	}

	return fmt.Sprintf("  payload:\n%s\n", pretty.String())
}
