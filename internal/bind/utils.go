package bind

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
)

func splitAndTrim(s string, seps string) []string {
	isSep := func(r rune) bool { return strings.ContainsRune(seps, r) }
	parts := strings.FieldsFunc(s, isSep) // 自动丢弃空片段
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}

func parseMultiValues(formats []string, rawValues []string, flag string) ([]string, error) {
	if rawValues == nil {
		return nil, nil
	}
	if len(rawValues) == 0 {
		return []string{}, nil
	}
	if len(formats) == 0 {
		return rawValues, nil
	}
	if checkInStringSlice("json", formats) {
		if len(formats) > 1 {
			return nil, fmt.Errorf("multi format 'json' for flag %s cannot be combined with other formats", flag)
		}
		var result []string
		for _, raw := range rawValues {
			raw = strings.TrimSpace(raw)
			if raw == "" {
				continue
			}
			// 尝试解析为 JSON 数组
			var arr []string
			if err := json.Unmarshal([]byte(raw), &arr); err == nil {
				result = append(result, arr...)
				continue
			}
			// 尝试解析为单个 JSON 字符串（"value"）
			var s string
			if err := json.Unmarshal([]byte(raw), &s); err == nil {
				result = append(result, s)
				continue
			} else {
				return nil, fmt.Errorf("invalid json multi value: %s for flag %s, %v", raw, flag, err)
			}
		}
		return result, nil
	} else {
		sepsBuilder := strings.Builder{}
		for _, format := range formats {
			switch format {
			case "comma":
				sepsBuilder.WriteString(",")
			case "newline":
				sepsBuilder.WriteString("\r\n")
			case "space":
				sepsBuilder.WriteString(" ")
			default:
				return nil, fmt.Errorf("unsupported multi format: %s for flag %s", format, flag)
			}
		}
		seps := sepsBuilder.String()
		var result []string
		for _, raw := range rawValues {
			splitValues := splitAndTrim(raw, seps)
			result = append(result, splitValues...)
		}
		return result, nil
	}
}

func outputMultiValues(formats []string, values []string) (string, error) {
	isJson := false
	if checkInStringSlice("json", formats) {
		if len(formats) > 1 {
			return "", fmt.Errorf("multi format 'json' cannot be combined with other formats")
		}
		isJson = true
	} else {
		isJson = false
	}
	if len(values) == 0 {
		if isJson {
			return "[]", nil
		} else {
			return "", nil
		}
	}
	if len(formats) == 0 {
		return strings.Join(values, ","), nil
	}
	if isJson {
		data, err := json.Marshal(values)
		if err != nil {
			return "", fmt.Errorf("failed to marshal multi values to json: %w", err)
		}
		return string(data), nil
	} else {
		var sep string
		if slices.Contains(formats, "comma") {
			sep = ","
		} else if slices.Contains(formats, "newline") {
			sep = "\n"
		} else if slices.Contains(formats, "space") {
			sep = " "
		} else {
			return "", fmt.Errorf("unsupported multi formats: %v", formats)
		}
		return strings.Join(values, sep), nil
	}
}
