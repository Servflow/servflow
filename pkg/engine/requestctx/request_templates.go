package requestctx

import (
	"fmt"
	"strings"
	"text/template"

	"github.com/asaskevich/govalidator"
	"golang.org/x/crypto/bcrypt"
)

func (rc *RequestContext) ConditionalTemplateFunctions() template.FuncMap {
	return template.FuncMap{
		"email":    rc.tmplFuncEmail,
		"empty":    rc.tmplFuncEmpty,
		"notempty": rc.tmplFuncNotEmpty,
		"bcrypt":   rc.tmplFuncBcrypt,
	}
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
	} else {
		return true, nil
	}
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
	} else {
		return true
	}
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
