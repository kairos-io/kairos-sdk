//go:build queryslim

package state

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

func (r Runtime) Query(s string) (res string, err error) {
	var parts []string
	for _, p := range strings.Split(s, ".") {
		for len(p) > 0 {
			if p[0] == '[' {
				end := strings.Index(p, "]")
				if end > 0 {
					idx := p[1:end]
					parts = append(parts, idx)
					p = p[end+1:]
					continue
				}
			}
			bracketIdx := strings.Index(p, "[")
			if bracketIdx > 0 {
				parts = append(parts, p[:bracketIdx])
				p = p[bracketIdx:]
				continue
			}
			parts = append(parts, p)
			break
		}
	}
	v := reflect.ValueOf(r)
	for _, part := range parts {
		// Dereference pointer if needed
		for v.Kind() == reflect.Ptr {
			v = v.Elem()
		}
		// If part is a number, treat as slice/array index
		if idx, err := strconv.Atoi(part); err == nil {
			if v.Kind() == reflect.Slice || v.Kind() == reflect.Array {
				if idx < 0 || idx >= v.Len() {
					return "", fmt.Errorf("invalid slice index '%s'", part)
				}
				v = v.Index(idx)
				continue
			}
		}
		switch v.Kind() {
		case reflect.Struct:
			found := false
			t := v.Type()
			for i := 0; i < t.NumField(); i++ {
				field := t.Field(i)
				jsonTag := field.Tag.Get("json")
				jsonName := strings.Split(jsonTag, ",")[0]
				if strings.EqualFold(field.Name, part) || (jsonName != "" && strings.EqualFold(jsonName, part)) {
					v = v.Field(i)
					found = true
					break
				}
			}
			if !found {
				return "", fmt.Errorf("field '%s' not found", part)
			}
		case reflect.Map:
			key := reflect.ValueOf(part)
			v = v.MapIndex(key)
			if !v.IsValid() {
				return "", fmt.Errorf("map key '%s' not found", part)
			}
		default:
			return "", fmt.Errorf("cannot traverse into %s", v.Kind())
		}
	}
	// Convert final value to string
	if v.Kind() == reflect.Ptr && !v.IsNil() {
		v = v.Elem()
	}
	if v.Kind() == reflect.String {
		return v.String(), nil
	}
	return fmt.Sprint(v.Interface()), nil
}
