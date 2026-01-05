package requestctx

import (
	"crypto/md5"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/Servflow/servflow/pkg/engine/secrets"
)

func getFuncMap(funcMap template.FuncMap) template.FuncMap {
	m := template.FuncMap{
		"strip":        tmplStripText,
		"jsonout":      jsonOut,
		"pluck":        tmplPluck,
		"escape":       stringEscape, // more idiomatic name
		"stringescape": stringEscape, // keep for backward compatibility
		"jsonraw":      jsonRaw,
		"join":         tmplJoin,
		"hash":         tmplHash,
		"now":          now,
		"secret":       secret,
	}
	for k, v := range funcMap {
		m[k] = v
	}

	return m
}

//var ConditionFunctionMaps = template.FuncMap{
//	"email":    tmplFuncEmail,
//	"empty":    tmplFuncEmpty,
//	"notempty": tmplNotEmpty,
//	"bcrypt":   tmplFuncBcrypt,
//	//"secretForKey": secretmanager.SecretForKey,
//	//"jsonout":      jsonOut,
//	//"escape":       stringEscape, // more idiomatic name
//	//"stringescape": stringEscape, // keep for backward compatibility
//	//"pluck":        tmplPluck,
//	//"join":         tmplJoin,
//	//"hash":         tmplHash,
//}

// stringEscape escapes a string to be safely used in various contexts
// by escaping special characters like quotes, backslashes, and control characters.
// Uses Go's native strconv.Quote and removes the surrounding quotes.
func stringEscape(s string) string {
	quoted := strconv.Quote(s)
	return quoted[1 : len(quoted)-1] // Remove surrounding quotes
}

func now() string {
	return time.Now().Format("2006-01-02 15:04:05")
}

func secret(key string) string {
	return secrets.FetchSecret(key)
}

func jsonRaw(val interface{}) string {
	dat, _ := json.Marshal(val)
	return string(dat)
}

func jsonOut(val interface{}) string {
	switch v := val.(type) {
	case string:
		return strings.Trim(strconv.Quote(v), "\"")
	default:
		dat, _ := json.Marshal(val)
		return string(dat) // json.Marshal already escapes properly
	}
}

func tmplStripText(text, toStrip string) string {
	text = strings.TrimPrefix(text, toStrip)
	text = strings.TrimSpace(text)
	return text
}

func tmplPluck(item any, key string) any {
	switch item := item.(type) {
	case map[string]interface{}:
		return item[key]
	case []map[string]interface{}:
		gotten := make([]any, 0)
		for _, item := range item {
			v, ok := item[key]
			if !ok {
				continue
			}
			gotten = append(gotten, v)
		}
		return gotten
	default:
		return item
	}
}

type ValidationError struct {
	err error
}

func (v *ValidationError) Error() string {
	if v.err == nil {
		return ""
	}
	return v.err.Error()
}

func (v *ValidationError) Unwrap() error {
	return v.err
}

// tmplHash generates an MD5 hash of the input.
// If the input is a string or primitive, it hashes it directly.
// If it's a complex type, it converts to JSON first, then hashes the JSON.
// Returns the hash as a hex-encoded string.
func tmplHash(item any) string {
	var data []byte

	switch v := item.(type) {
	case string:
		data = []byte(v)
	case int, int64, int32, int16, int8, uint, uint64, uint32, uint16, uint8, float64, float32, bool:
		// For primitives, convert to string then hash
		data = []byte(fmt.Sprintf("%v", v))
	default:
		// For complex types, marshal to JSON first
		jsonData, err := json.Marshal(v)
		if err != nil {
			// Return empty string if marshaling fails
			return ""
		}
		data = jsonData
	}

	// Generate MD5 hash
	hash := md5.Sum(data)
	// Return hex encoded hash
	return fmt.Sprintf("%x", hash)
}

// tmplJoin joins an array of strings with the specified separator.
// If the input is not an array of strings, it returns the input as is.
func tmplJoin(item any, sep string) any {
	switch v := item.(type) {
	case []string:
		return strings.Join(v, sep)
	case []interface{}:
		// Try to convert each item to string
		strSlice := make([]string, 0, len(v))
		for _, val := range v {
			if str, ok := val.(string); ok {
				strSlice = append(strSlice, str)
			} else {
				// If any item is not a string, return original
				return item
			}
		}
		return strings.Join(strSlice, sep)
	default:
		return item
	}
}
