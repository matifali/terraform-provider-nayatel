// Copyright (c) 2026 Muhammad Atif Ali
// SPDX-License-Identifier: MPL-2.0

package client

import (
	"encoding/json"
	"fmt"
)

// decodeList decodes an API list response that is either a bare JSON array or
// an object wrapping the array under one of the given keys. An object without
// any of the keys — such as an error payload served with HTTP 200 — is an
// error rather than an empty list, so callers never mistake a failure for
// "no items" (which would make resources vanish from Terraform state).
func decodeList[T any](resp []byte, keys ...string) ([]T, error) {
	var items []T
	if err := json.Unmarshal(resp, &items); err == nil {
		return items, nil
	}

	var obj map[string]json.RawMessage
	if err := json.Unmarshal(resp, &obj); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	for _, key := range keys {
		raw, ok := obj[key]
		if !ok {
			continue
		}
		if err := json.Unmarshal(raw, &items); err != nil {
			return nil, fmt.Errorf("failed to decode %q field: %w", key, err)
		}
		return items, nil
	}

	// Some Nayatel list endpoints report an empty collection as a bare
	// {"message": "No <things> found."} object instead of an empty array
	// under the expected key. That shape is unambiguous only when "message"
	// is the object's sole field — anything else present (e.g. "status" or
	// "error") keeps this an error, matching a real failure payload.
	if raw, ok := obj["message"]; ok && len(obj) == 1 {
		var message string
		if err := json.Unmarshal(raw, &message); err == nil {
			return nil, nil
		}
	}

	return nil, fmt.Errorf("unexpected response shape (no %v field): %s", keys, truncateForError(resp))
}

func truncateForError(b []byte) string {
	const limit = 300
	if len(b) > limit {
		return string(b[:limit]) + "..."
	}
	return string(b)
}
