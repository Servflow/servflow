package plan

import (
	"testing"

	"github.com/Servflow/servflow/pkg/apiconfig"
	httpresp "github.com/Servflow/servflow/pkg/engine/responses/http"
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
				Type:     "template",
				Template: "",
				Code:     200,
			},
			assertBuilder: func(t *testing.T, response *Response) {
				_, ok := response.responseBuilder.(*httpresp.TemplateBuilder)
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
				_, ok := response.responseBuilder.(*httpresp.JSONObjectBuilder)
				if !ok {
					t.Errorf("Response builder is not a JSONObjectBuilder, it is %T", response.responseBuilder)
				}
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			gotten, err := newResponse(tc.id, tc.id, tc.config)
			if tc.expectedErr {
				assert.Error(t, err)
				return
			}
			tc.assertBuilder(t, gotten)
		})
	}
}
