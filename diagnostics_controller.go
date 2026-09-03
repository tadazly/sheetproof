package main

import (
	"fmt"
	"sort"
	"strings"
)

const (
	maxFrontendDiagnosticEvents = 80
	maxFrontendDiagnosticFields = 32
	maxFrontendDiagnosticText   = 240
)

type FrontendDiagnosticEvent struct {
	ID        string         `json:"id"`
	Timestamp string         `json:"timestamp"`
	Name      string         `json:"name"`
	Fields    map[string]any `json:"fields"`
}

func (c *Controller) RecordDiagnostics(events []FrontendDiagnosticEvent) {
	c.mu.Lock()
	log := c.diagnosticLog
	c.mu.Unlock()
	if log == nil || len(events) == 0 {
		return
	}
	if len(events) > maxFrontendDiagnosticEvents {
		events = events[len(events)-maxFrontendDiagnosticEvents:]
	}
	for _, event := range events {
		fields := sanitizeFrontendDiagnosticFields(event.Fields)
		fields["frontendId"] = truncateFrontendDiagnosticText(event.ID)
		fields["frontendTimestamp"] = truncateFrontendDiagnosticText(event.Timestamp)
		log.Record("frontend."+event.Name, fields)
	}
}

func sanitizeFrontendDiagnosticFields(values map[string]any) map[string]any {
	result := make(map[string]any)
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	if len(keys) > maxFrontendDiagnosticFields {
		keys = keys[:maxFrontendDiagnosticFields]
	}
	for _, key := range keys {
		safeKey := sanitizeFrontendDiagnosticKey(key)
		if safeKey == "" {
			continue
		}
		switch value := values[key].(type) {
		case nil, bool, float64, float32, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
			result[safeKey] = value
		case string:
			result[safeKey] = truncateFrontendDiagnosticText(value)
		default:
			result[safeKey] = fmt.Sprintf("unsupported:%T", value)
		}
	}
	return result
}

func sanitizeFrontendDiagnosticKey(value string) string {
	var result strings.Builder
	for _, character := range value {
		if character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' ||
			character >= '0' && character <= '9' || character == '_' || character == '-' {
			result.WriteRune(character)
		} else {
			result.WriteByte('_')
		}
		if result.Len() >= 64 {
			break
		}
	}
	return strings.Trim(result.String(), "_")
}

func truncateFrontendDiagnosticText(value string) string {
	characters := []rune(value)
	if len(characters) <= maxFrontendDiagnosticText {
		return value
	}
	return string(characters[:maxFrontendDiagnosticText])
}
