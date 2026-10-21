package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"catbrasil/backend/internal/money"
)

const maxAdminJSONBody = 4 << 20

// decodeExactMonetaryJSON validates monetary lexemes before float64 DTO fields
// can erase sub-cent digits. Paths can traverse arrays with [] (catalog[].price).
func decodeExactMonetaryJSON(body io.Reader, target any, paths ...string) error {
	data, err := io.ReadAll(io.LimitReader(body, maxAdminJSONBody+1))
	if err != nil {
		return err
	}
	if len(data) > maxAdminJSONBody {
		return fmt.Errorf("payload administrativo excede o limite")
	}
	if err := json.Unmarshal(data, target); err != nil {
		return err
	}
	for _, path := range paths {
		if err := validateMonetaryJSONPath(data, strings.Split(path, ".")); err != nil {
			return fmt.Errorf("campo %s: %w", path, err)
		}
	}
	return nil
}

func validateMonetaryJSONPath(raw json.RawMessage, path []string) error {
	if len(path) == 0 {
		_, err := money.ParseJSONNumber(string(raw))
		return err
	}
	segment := path[0]
	isArray := strings.HasSuffix(segment, "[]")
	if isArray {
		segment = strings.TrimSuffix(segment, "[]")
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return err
	}
	child, ok := fields[segment]
	if !ok {
		return nil
	}
	if !isArray {
		return validateMonetaryJSONPath(child, path[1:])
	}
	var items []json.RawMessage
	if err := json.Unmarshal(child, &items); err != nil {
		return err
	}
	for _, item := range items {
		if err := validateMonetaryJSONPath(item, path[1:]); err != nil {
			return err
		}
	}
	return nil
}
