package pricedifference

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
		wantErr   bool
	}{
		{
			name:   "single symbol",
			config: Config{Symbol: "BTCUSDT", Interval: "1h", Period: "24"},
			expect: func(m *binance.MockIntegration) {
				m.EXPECT().GetCurrentPrice("BTCUSDT").Return(50000.0, nil)
				m.EXPECT().GetHistoricalPrice("BTCUSDT", "1h", 24).Return([]*binance.KlineData{{Close: "48000.00"}}, nil)
			},
		},
		{
			name:   "multiple symbols",
			config: Config{Symbol: "BTCUSDT,ETHUSDT", Interval: "1h", Period: "24"},
			expect: func(m *binance.MockIntegration) {
				m.EXPECT().GetCurrentPrice("BTCUSDT").Return(50000.0, nil)
				m.EXPECT().GetHistoricalPrice("BTCUSDT", "1h", 24).Return([]*binance.KlineData{{Close: "48000.00"}}, nil)
				m.EXPECT().GetCurrentPrice("ETHUSDT").Return(3000.0, nil)
				m.EXPECT().GetHistoricalPrice("ETHUSDT", "1h", 24).Return([]*binance.KlineData{{Close: "2900.00"}}, nil)
			},
		},
		{
			name:      "templated fields resolve against the request",
			config:    Config{Symbol: "{{ .symbol }}", Interval: "{{ .interval }}", Period: "{{ .period }}"},
			variables: map[string]any{"symbol": "BTC\"USDT\"", "interval": "4h", "period": "6"},
			expect: func(m *binance.MockIntegration) {
				m.EXPECT().GetCurrentPrice("BTC\"USDT\"").Return(50000.0, nil)
				m.EXPECT().GetHistoricalPrice("BTC\"USDT\"", "4h", 6).Return([]*binance.KlineData{{Close: "48000.00"}}, nil)
			},
		},
		{
			name:   "api error",
			config: Config{Symbol: "BTCUSDT", Interval: "1h", Period: "24"},
			expect: func(m *binance.MockIntegration) {
				m.EXPECT().GetCurrentPrice("BTCUSDT").Return(0.0, assert.AnError)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := binance.NewMockIntegration(gomock.NewController(t))
			tt.expect(mock)
			rc := requestctxtest.New()
			rc.SetIntegration("binance", mock)
			rc.AddVariables(tt.variables, "")

			cfg := tt.config
			cfg.Integration = "binance"
			executable := &Executable{config: cfg}
			_, _, err := executable.Execute(requestctx.With(context.Background(), rc))
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}
