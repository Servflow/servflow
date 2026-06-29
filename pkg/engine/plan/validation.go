package plan

// Config validation lives in the plan package because the planner and the
// validator both interpret the same step graph (via apiconfig.ParseStepRef) and
// the same action registry; keeping them together stops them drifting. apiconfig
// itself stays struct-only and depends on no registry.

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/Servflow/servflow/pkg/apiconfig"
	"github.com/Servflow/servflow/pkg/engine/actions"
	"github.com/Servflow/servflow/pkg/schemavalidate"
)

//go:embed apiconfig_schema.json
var apiConfigSchema string

// compiledAPIConfigSchema is the embedded APIConfig schema, compiled once.
var compiledAPIConfigSchema = schemavalidate.MustCompileSchema("apiconfig.json", apiConfigSchema)

// Validate runs the full validation cascade over an APIConfig.
func Validate(a *apiconfig.APIConfig) error {
	return ValidateWithEntries(a)
}

// ValidateWithEntries validates the config, treating each extra root as an
// additional entry point into the step graph (used by the pro layer to feed a
// trigger's start step, which the engine config has no other knowledge of).
func ValidateWithEntries(a *apiconfig.APIConfig, extraRoots ...string) error {
	var validationErrors ValidationErrors

	collectSchemaErrors(a, &validationErrors)
	collectActionErrors(a, &validationErrors)
	collectGraphErrors(a, &validationErrors, extraRoots)

	if validationErrors.HasErrors() {
		return &validationErrors
	}
	return nil
}

type ActionConfigError struct {
	ActionID string
	// Field is the offending config field, when the error is field-specific.
	Field   string
	Message string
}

func (e *ActionConfigError) Error() string {
	return fmt.Sprintf("action '%s': %s", e.ActionID, e.Message)
}

type ValidationErrors struct {
	errors   []error
	warnings []error
}

func (ve *ValidationErrors) Error() string {
	if len(ve.errors) == 0 {
		return ""
	}
	var lines []string
	for _, err := range ve.errors {
		lines = append(lines, err.Error())
	}
	return strings.Join(lines, "\n")
}

func (ve *ValidationErrors) Add(err error) {
	ve.errors = append(ve.errors, err)
}

// AddWarning records a non-fatal finding. Warnings do not count towards
// HasErrors, so a config with only warnings still validates.
func (ve *ValidationErrors) AddWarning(err error) {
	ve.warnings = append(ve.warnings, err)
}

// Warnings returns the non-fatal findings collected during validation.
func (ve *ValidationErrors) Warnings() []error {
	return ve.warnings
}

func (ve *ValidationErrors) HasErrors() bool {
	return len(ve.errors) > 0
}

func (ve *ValidationErrors) GetActionConfigErrors() []*ActionConfigError {
	var actionErrors []*ActionConfigError
	for _, err := range ve.errors {
		var actionErr *ActionConfigError
		if errors.As(err, &actionErr) {
			actionErrors = append(actionErrors, actionErr)
		}
	}
	return actionErrors
}

func (ve *ValidationErrors) GetSchemaValidationErrors() []*schemavalidate.SchemaValidationError {
	var schemaErrors []*schemavalidate.SchemaValidationError
	for _, err := range ve.errors {
		var schemaErr *schemavalidate.SchemaValidationError
		if errors.As(err, &schemaErr) {
			schemaErrors = append(schemaErrors, schemaErr)
		}
	}
	return schemaErrors
}

func collectSchemaErrors(a *apiconfig.APIConfig, validationErrors *ValidationErrors) {
	configJSON, err := json.Marshal(a)
	if err != nil {
		validationErrors.Add(fmt.Errorf("failed to marshal APIConfig to JSON: %w", err))
		return
	}
	schemaErrs, err := schemavalidate.ValidateInstance(compiledAPIConfigSchema, configJSON, locate)
	if err != nil {
		validationErrors.Add(fmt.Errorf("schema validation failed: %w", err))
		return
	}
	for _, se := range schemaErrs {
		validationErrors.Add(se)
	}
}

// locate maps an instance location (path tokens) to a domain noun for the
// engine APIConfig schema:
//
//	["actions","createUser"]        -> action "createUser"
//	["actions","createUser","type"] -> action "createUser" field "type"
//	["responses","ok","code"]       -> response "ok" field "code"
//	[]                              -> config
func locate(tokens []string) string {
	if len(tokens) == 0 {
		return "config"
	}
	nouns := map[string]string{
		"actions":      "action",
		"conditionals": "conditional",
		"responses":    "response",
		"integrations": "integration",
	}
	if noun, ok := nouns[tokens[0]]; ok && len(tokens) >= 2 {
		base := fmt.Sprintf("%s %q", noun, tokens[1])
		if len(tokens) > 2 {
			return fmt.Sprintf("%s field %q", base, strings.Join(tokens[2:], "."))
		}
		return base
	}
	return fmt.Sprintf("field %q", strings.Join(tokens, "."))
}

func collectActionErrors(a *apiconfig.APIConfig, validationErrors *ValidationErrors) {
	for actionID, action := range a.Actions {
		if !actions.HasRegisteredActionType(action.Type) {
			validationErrors.Add(&ActionConfigError{
				ActionID: actionID,
				Message:  fmt.Sprintf("invalid action type: %s", action.Type),
			})
			continue
		}

		fields, err := actions.GetFieldsForAction(action.Type)
		if err != nil {
			validationErrors.Add(&ActionConfigError{
				ActionID: actionID,
				Message:  err.Error(),
			})
			continue
		}
		for _, field := range missingRequiredFields(fields, action.Config) {
			validationErrors.Add(&ActionConfigError{
				ActionID: actionID,
				Field:    field,
				Message:  fmt.Sprintf("missing required field %q", field),
			})
		}
	}
}

// missingRequiredFields returns the names of required fields that are absent or
// empty in the action config. We only check presence, not type: a field holding
// a Go template (e.g. "{{ body \"x\" }}") is a non-empty string and so counts as
// provided. Field names are sorted for stable, deterministic error output.
func missingRequiredFields(fields map[string]actions.FieldInfo, values map[string]interface{}) []string {
	var missing []string
	for name, info := range fields {
		if !info.Required {
			continue
		}
		if isEmptyFieldValue(values[name]) {
			missing = append(missing, name)
		}
	}
	sort.Strings(missing)
	return missing
}

// isEmptyFieldValue reports whether a config value should count as "not
// provided". A missing key (nil) and an empty string are empty; any other
// present value counts as provided.
func isEmptyFieldValue(val interface{}) bool {
	if val == nil {
		return true
	}
	if s, ok := val.(string); ok {
		return s == ""
	}
	return false
}
