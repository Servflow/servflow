package apiconfig

import "testing"

func TestParseStepRef(t *testing.T) {
	tests := []struct {
		name     string
		raw      string
		wantKind StepKind
		wantID   string
		wantTerm bool
		wantErr  bool
	}{
		{"empty is terminal", "", StepKindUnknown, "", true, false},
		{"dollar empty is terminal", "$", StepKindUnknown, "", true, false},
		{"action ref", "action.createUser", StepKindAction, "createUser", false, false},
		{"conditional ref", "conditional.isValid", StepKindConditional, "isValid", false, false},
		{"response ref", "response.ok", StepKindResponse, "ok", false, false},
		{"dollar prefix stripped", "$action.foo", StepKindAction, "foo", false, false},
		{"bare word is error", "end", StepKindUnknown, "end", false, true},
		{"unknown prefix is error", "step.foo", StepKindUnknown, "step.foo", false, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kind, id, term, err := ParseStepRef(tt.raw)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr = %v", err, tt.wantErr)
			}
			if kind != tt.wantKind || id != tt.wantID || term != tt.wantTerm {
				t.Fatalf("got (kind=%v id=%q term=%v), want (kind=%v id=%q term=%v)",
					kind, id, term, tt.wantKind, tt.wantID, tt.wantTerm)
			}
		})
	}
}
