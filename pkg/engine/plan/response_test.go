package plan

import (
	"testing"

	"github.com/Servflow/servflow/pkg/apiconfig"
	responsebuilder2 "github.com/Servflow/servflow/pkg/engine/plan/responsebuilder"
	"github.com/stretchr/testify/assert"
)

func TestNewResponse(t *testing.T) {
	testCases := []struct {
		name          string
		id            string
		config        apiconfig.ResponseConfig
		expectedErr   bool
		assertBuilder func(*testing.T, *Response)
	}{
		{
			name: "json",
			id:   "id",
			config: apiconfig.ResponseConfig{
				Type:     builderTypeTemplate,
				Template: "",
				Code:     200,
			},
			assertBuilder: func(t *testing.T, response *Response) {
				_, ok := response.responseBuilder.(*responsebuilder2.TemplateBuilder)
				if !ok {
					t.Errorf("Response builder is not a TemplateBuilder")
				}
			},
		},
		{
			name: "default object",
			id:   "id",
			config: apiconfig.ResponseConfig{
				Type:     "",
				Code:     200,
				Template: "",
				Object: apiconfig.ResponseObject{
					Fields: map[string]apiconfig.ResponseObject{
						"status": {
							Value: "test",
						},
					},
				},
			},
			assertBuilder: func(t *testing.T, response *Response) {
				_, ok := response.responseBuilder.(*responsebuilder2.JSONObjectBuilder)
				if !ok {
					t.Errorf("Response builder is not a JSONObjectBuilder, it is %T", response.responseBuilder)
				}
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			gotten, err := newResponse(tc.id, tc.config)
			if tc.expectedErr {
				assert.Error(t, err)
				return
			}
			tc.assertBuilder(t, gotten)
		})
	}
}
