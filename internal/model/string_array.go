package model

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"strings"
)

type StringArray []string

func (a *StringArray) Scan(src any) error {
	if src == nil {
		*a = nil
		return nil
	}

	switch v := src.(type) {
	case string:
		parsed, err := parseStringArray(v)
		if err != nil {
			return err
		}
		*a = StringArray(parsed)
		return nil
	case []byte:
		parsed, err := parseStringArray(string(v))
		if err != nil {
			return err
		}
		*a = StringArray(parsed)
		return nil
	case []string:
		*a = StringArray(v)
		return nil
	default:
		return fmt.Errorf("unsupported Scan, storing driver.Value type %T into type *model.StringArray", src)
	}
}

func (a StringArray) Value() (driver.Value, error) {
	if a == nil {
		return nil, nil
	}

	var b strings.Builder
	b.WriteByte('{')
	for i, item := range a {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteByte('"')
		b.WriteString(escapeArrayElement(item))
		b.WriteByte('"')
	}
	b.WriteByte('}')
	return b.String(), nil
}

func parseStringArray(raw string) ([]string, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return []string{}, nil
	}

	if strings.HasPrefix(s, "[") {
		var values []string
		if err := json.Unmarshal([]byte(s), &values); err != nil {
			return nil, err
		}
		return values, nil
	}

	if !(strings.HasPrefix(s, "{") && strings.HasSuffix(s, "}")) {
		return nil, fmt.Errorf("unsupported array format: %q", s)
	}

	content := s[1 : len(s)-1]
	if content == "" {
		return []string{}, nil
	}

	result := make([]string, 0, 4)
	var current strings.Builder
	inQuotes := false
	escaped := false

	for _, r := range content {
		switch {
		case escaped:
			current.WriteRune(r)
			escaped = false
		case r == '\\':
			escaped = true
		case r == '"':
			inQuotes = !inQuotes
		case r == ',' && !inQuotes:
			result = append(result, current.String())
			current.Reset()
		default:
			current.WriteRune(r)
		}
	}

	if escaped || inQuotes {
		return nil, fmt.Errorf("malformed array literal: %q", s)
	}

	result = append(result, current.String())
	for i := range result {
		result[i] = strings.TrimSpace(result[i])
		if strings.EqualFold(result[i], "NULL") {
			result[i] = ""
		}
	}
	return result, nil
}

func escapeArrayElement(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `"`, `\"`)
	return value
}
