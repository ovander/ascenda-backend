package service

import "encoding/json"

// marshalChanges serialises a payload map/struct for the audit event Changes
// field.  It silently returns nil on marshal failure so that audit enrichment
// never breaks a business-logic call path.
func marshalChanges(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	return b
}

// countUnique counts the number of distinct string values produced by key(item)
// across a slice.  Used to derive category/line counts for audit payloads.
func countUnique[T any](items []T, key func(T) string) int {
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		seen[key(item)] = struct{}{}
	}
	return len(seen)
}
