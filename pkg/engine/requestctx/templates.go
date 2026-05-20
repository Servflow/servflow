package requestctx

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/Servflow/servflow/pkg/engine/secrets"
	"github.com/asaskevich/govalidator"
	"golang.org/x/crypto/bcrypt"
)

// batchSeparator is used for batch resolution - uses characters that won't appear in normal data
const batchSeparator = "\x00\x1F\x00" // Null + Unit Separator + Null

// Resolve resolves a single template string using the request context
func (rc *RequestContext) Resolve(ctx context.Context, templateStr string) (string, error) {
	tmpl, err := rc.createTemplate(templateStr, nil)
	if err != nil {
		return "", fmt.Errorf("creating template: %w", err)
	}
	return rc.executeTemplate(tmpl)
}

// ResolveBatch resolves multiple templates efficiently by concatenating,
// resolving once, then splitting. This reduces template parsing overhead.
func (rc *RequestContext) ResolveBatch(ctx context.Context, templates ...string) ([]string, error) {
	if len(templates) == 0 {
		return []string{}, nil
	}
	if len(templates) == 1 {
		result, err := rc.Resolve(ctx, templates[0])
		if err != nil {
			return nil, err
		}
		return []string{result}, nil
	}

	// Concatenate all templates with separator
	combined := strings.Join(templates, batchSeparator)

	// Single resolution pass
	resolved, err := rc.Resolve(ctx, combined)
	if err != nil {
		return nil, err
	}

	// Split back into individual results
	results := strings.Split(resolved, batchSeparator)
	if len(results) != len(templates) {
		return nil, fmt.Errorf("batch resolution mismatch: expected %d results, got %d",
			len(templates), len(results))
	}

	return results, nil
}

// createTemplate creates a template with all functions available
func (rc *RequestContext) createTemplate(in string, funcMap template.FuncMap) (*template.Template, error) {
	funcMap = rc.getFuncMap(funcMap)
	replaced := replaceEscapedQuotes(in)
	replaced = normalizeActionVariables(replaced)
	return template.New("input").Option("missingkey=zero").Funcs(funcMap).Parse(replaced)
}

// executeTemplate executes a template against the request variables
func (rc *RequestContext) executeTemplate(tmpl *template.Template) (string, error) {
	rc.Lock()
	vars := make(map[string]interface{}, len(rc.requestVariables))
	for k, v := range rc.requestVariables {
		vars[k] = v
	}
	rc.Unlock()

	var buff bytes.Buffer
	if err := tmpl.Execute(&buff, vars); err != nil {
		return "", fmt.Errorf("error processing template: %w", err)
	}

	return strings.ReplaceAll(buff.String(), noValue, ""), nil
}

// getFuncMap returns all template functions including request-scoped ones
func (rc *RequestContext) getFuncMap(funcMap template.FuncMap) template.FuncMap {
	m := template.FuncMap{
		// Base functions
		"strip":        tmplStripText,
		"jsonout":      jsonOut,
		"pluck":        tmplPluck,
		"escape":       stringEscape,
		"stringescape": stringEscape, // backward compatibility
		"jsonraw":      jsonRaw,
		"join":         tmplJoin,
		"hash":         tmplHash,
		"now":          now,
		"secret":       secret,
		"email":        rc.tmplFuncEmail,
		"empty":        rc.tmplFuncEmpty,
		"notempty":     rc.tmplFuncNotEmpty,
		"bcrypt":       rc.tmplFuncBcrypt,
	}
	// Add request-scoped functions (param, header, body, urlparam, etc.)
	for k, v := range rc.requestFuncs {
		m[k] = v
	}
	// Add any additional functions provided by caller
	for k, v := range funcMap {
		m[k] = v
	}
	return m
}

// stringEscape escapes a string to be safely used in various contexts
// by escaping special characters like quotes, backslashes, and control characters.
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
		return string(dat)
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

// tmplHash generates an MD5 hash of the input.
func tmplHash(item any) string {
	var data []byte

	switch v := item.(type) {
	case string:
		data = []byte(v)
	case int, int64, int32, int16, int8, uint, uint64, uint32, uint16, uint8, float64, float32, bool:
		data = []byte(fmt.Sprintf("%v", v))
	default:
		jsonData, err := json.Marshal(v)
		if err != nil {
			return ""
		}
		data = jsonData
	}

	hash := md5.Sum(data)
	return fmt.Sprintf("%x", hash)
}

// tmplJoin joins an array of strings with the specified separator.
func tmplJoin(item any, sep string) any {
	switch v := item.(type) {
	case []string:
		return strings.Join(v, sep)
	case []interface{}:
		strSlice := make([]string, 0, len(v))
		for _, val := range v {
			if str, ok := val.(string); ok {
				strSlice = append(strSlice, str)
			} else {
				return item
			}
		}
		return strings.Join(strSlice, sep)
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

func (rc *RequestContext) tmplFuncEmail(email interface{}, title string) bool {
	s, ok := email.(string)
	if !ok {
		rc.validationErrors = append(rc.validationErrors, fmt.Errorf("%s is not a valid email address", title))
		return false
	}
	if govalidator.IsEmail(s) {
		return true
	}
	rc.validationErrors = append(rc.validationErrors, fmt.Errorf("%s is not a valid email address", title))
	return false
}

func (rc *RequestContext) tmplFuncEmpty(item interface{}, title string) (bool, error) {
	if item == nil {
		return true, nil
	}
	var pass bool
	switch t := item.(type) {
	case map[string]interface{}:
		pass = len(t) == 0
	case []interface{}:
		pass = len(t) == 0
	case []map[string]interface{}:
		pass = len(t) == 0
	case string:
		pass = t == ""
	default:
		return false, fmt.Errorf("%s is not a valid type", title)
	}

	if !pass {
		rc.validationErrors = append(rc.validationErrors, fmt.Errorf("%s should be empty", title))
		return false, nil
	}
	return true, nil
}

func (rc *RequestContext) tmplFuncNotEmpty(item interface{}, title string) bool {
	pass := false
	if item != nil {
		switch t := item.(type) {
		case map[string]interface{}:
			pass = len(t) > 0
		case []interface{}:
			pass = len(t) > 0
		case []map[string]interface{}:
			pass = len(t) > 0
		case string:
			pass = t != "null" && t != ""
		default:
			pass = false
		}
	}
	if !pass {
		rc.validationErrors = append(rc.validationErrors, fmt.Errorf("%s can not be empty", title))
		return false
	}
	return true
}

func (rc *RequestContext) tmplFuncBcrypt(val, hashed, name string) bool {
	hashed = strings.TrimSpace(hashed)
	err := bcrypt.CompareHashAndPassword([]byte(hashed), []byte(val))
	if err != nil {
		rc.validationErrors = append(rc.validationErrors, fmt.Errorf("%s does not match", name))
		return false
	}
	return true
}
