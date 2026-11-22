package getprice

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/Servflow/servflow/pkg/binance"
	"github.com/Servflow/servflow/pkg/engine/requestctx"
	"github.com/Servflow/servflow/pkg/engine/requestctx/requestctxtest"
)

func TestExecute(t *testing.T) {
	tests := []struct {
		name      string
		config    Config
		variables map[string]any
		expect    func(m *binance.MockIntegration)
		wantErr   string
	}{
		{
			name:   "current price",
			config: Config{Symbol: "BTCUSDT", PriceType: "current"},
			expect: func(m *binance.MockIntegration) {
				m.EXPECT().GetCurrentPrice("BTCUSDT").Return(50000.0, nil)
			},
		},
		{
			name:   "24hr ticker",
			config: Config{Symbol: "ETHUSDT", PriceType: "24hr"},
			expect: func(m *binance.MockIntegration) {
				m.EXPECT().Get24HrTicker("ETHUSDT").Return(&binance.TickerStats{}, nil)
			},
		},
		{
			name:      "templated fields resolve against the request",
			config:    Config{Symbol: "{{ .symbol }}", PriceType: "{{ .price_type }}"},
			variables: map[string]any{"symbol": "BTC\"USDT\"\nX", "price_type": "current"},
			expect: func(m *binance.MockIntegration) {
				m.EXPECT().GetCurrentPrice("BTC\"USDT\"\nX").Return(50000.0, nil)
			},
		},
		{
			name:    "empty symbol",
			config:  Config{Symbol: "", PriceType: "current"},
			wantErr: "symbol is required",
		},
		{
			name:    "invalid price type",
			config:  Config{Symbol: "BTCUSDT", PriceType: "invalid"},
			wantErr: "invalid price_type",
		},
		{
			name:   "api error",
			config: Config{Symbol: "BTCUSDT", PriceType: "current"},
			expect: func(m *binance.MockIntegration) {
				m.EXPECT().GetCurrentPrice("BTCUSDT").Return(0.0, assert.AnError)
			},
			wantErr: assert.AnError.Error(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := binance.NewMockIntegration(gomock.NewController(t))
			if tt.expect != nil {
				tt.expect(mock)
			}
			rc := requestctxtest.New()
			rc.AddVariables(tt.variables, "")

			e := &Executable{config: tt.config, integration: mock}
			_, _, err := e.Execute(requestctx.With(context.Background(), rc))
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)
		})
	}
}
