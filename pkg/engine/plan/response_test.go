package plan

import (
	"testing"

	"github.com/Servflow/servflow/pkg/definitions"
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
				BuilderType: builderTypeJSON,
				Template:    "",
				Code:        200,
			},
			assertBuilder: func(t *testing.T, response *Response) {
				_, ok := response.responseBuilder.(*responsebuilder2.JSONResponseBuilder)
				if !ok {
					t.Errorf("Response builder is not a JSONResponseBuilder")
				}
			},
		},
		{
			name: "default object",
			id:   "id",
			config: apiconfig.ResponseConfig{
				BuilderType: "",
				Code:        200,
				Template:    "",
				Object: apiconfig.ResponseObject{
					Fields: map[string]apiconfig.ResponseObject{
						"status": {
							Value: "test",
						},
					},
				},
			},
			assertBuilder: func(t *testing.T, response *Response) {
				_, ok := response.responseBuilder.(*responsebuilder2.ObjectBuilder)
				if !ok {
					t.Errorf("Response builder is not a ObjectBuilder, it is %T", response.responseBuilder)
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
