package tradeinfo

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
			name:   "spot open orders",
			config: Config{Symbol: "BTCUSDT"},
			expect: func(m *binance.MockIntegration) {
				m.EXPECT().GetOpenOrders("BTCUSDT").Return(&binance.OpenOrdersResponse{}, nil)
			},
		},
		{
			name:   "spot order info",
			config: Config{Symbol: "BTCUSDT", OrderID: "12345"},
			expect: func(m *binance.MockIntegration) {
				m.EXPECT().GetOrderInfo("BTCUSDT", int64(12345)).Return(&binance.OrderInfo{}, nil)
			},
		},
		{
			name:   "futures open orders",
			config: Config{Symbol: "BTCUSDT", Futures: true},
			expect: func(m *binance.MockIntegration) {
				m.EXPECT().GetFuturesOpenOrders("BTCUSDT").Return(&binance.FuturesOpenOrdersResponse{}, nil)
			},
		},
		{
			name:   "futures positions",
			config: Config{Symbol: "BTCUSDT", Futures: true, InfoType: InfoTypePositions},
			expect: func(m *binance.MockIntegration) {
				m.EXPECT().GetAllPositions("BTCUSDT").Return(&binance.PositionsResponse{}, nil)
			},
		},
		{
			name:      "templated fields resolve against the request",
			config:    Config{Symbol: "{{ .symbol }}", OrderID: "{{ .order_id }}"},
			variables: map[string]any{"symbol": "BTC\"USDT\"\nX", "order_id": "777"},
			expect: func(m *binance.MockIntegration) {
				m.EXPECT().GetOrderInfo("BTC\"USDT\"\nX", int64(777)).Return(&binance.OrderInfo{}, nil)
			},
		},
		{
			name:   "api error",
			config: Config{Symbol: "BTCUSDT"},
			expect: func(m *binance.MockIntegration) {
				m.EXPECT().GetOpenOrders("BTCUSDT").Return(nil, assert.AnError)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := binance.NewMockIntegration(gomock.NewController(t))
			tt.expect(mock)
			rc := requestctxtest.New()
			rc.AddVariables(tt.variables, "")

			executable := &Executable{config: tt.config, integration: mock}
			_, _, err := executable.Execute(requestctx.With(context.Background(), rc))
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}
