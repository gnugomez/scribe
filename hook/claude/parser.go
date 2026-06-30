// Copyright (c) 2026 Eclipse Foundation AISBL
// 
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// http://www.eclipse.org/legal/epl-2.0.
// 
// SPDX-License-Identifier: EPL-2.0

package hook

import (
	"bufio"
	"encoding/json"
	"io"
	"time"

	registry "github.com/gnugomez/scribe/hook"
	"github.com/gnugomez/scribe/store"
)

const vendor = "anthropic"

func init() { registry.Register(&Parser{}) }

type Parser struct{}

func (p *Parser) Name() string { return "claude" }

type payload struct {
	HookEventName string `json:"hook_event_name"`
	Model         string `json:"model"`
	SessionID     string `json:"session_id"`
}

func (p *Parser) Parse(r io.Reader, _, model string) ([]store.Entry, error) {
	flagModel := model
	fallbackModel := model
	if fallbackModel == "" {
		fallbackModel = p.Name()
	}

	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		var pl payload
		_ = json.Unmarshal([]byte(line), &pl)

		resolvedModel, modelSource := resolveModel(pl.Model, flagModel, fallbackModel)
		return []store.Entry{{
			Timestamp:    time.Now().UTC(),
			Vendor:       vendor,
			Model:        resolvedModel,
			ModelSource:  modelSource,
			SessionID: pl.SessionID,
			EventName: pl.HookEventName,
			Payload:   line,
		}}, scanner.Err()
	}
	return nil, scanner.Err()
}

func resolveModel(fromPayload, flagFallback, defaultFallback string) (string, string) {
	if fromPayload != "" {
		return fromPayload, "payload"
	}
	if flagFallback != "" {
		return flagFallback, "flag"
	}
	return defaultFallback, "default"
}
