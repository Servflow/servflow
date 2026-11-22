package futuresorder

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/Servflow/servflow/pkg/binance"
	"github.com/Servflow/servflow/pkg/engine/requestctx"
	"github.com/Servflow/servflow/pkg/engine/requestctx/requestctxtest"
)

func TestExecute(t *testing.T) {
	price := 45000.0
	tests := []struct {
		name      string
		config    Config
		variables map[string]any
		expect    func(m *binance.MockIntegration)
	}{
		{
			name:   "market order",
			config: Config{Symbol: "BTCUSDT", Side: "BUY", Quantity: "0.001", Type: "MARKET"},
			expect: func(m *binance.MockIntegration) {
				m.EXPECT().PlaceFuturesOrder("BTCUSDT", "BUY", 0.001, "MARKET", (*float64)(nil), "", "", false).
					Return(&binance.FuturesOrderResponse{OrderID: 12345, Symbol: "BTCUSDT", Status: "FILLED"}, nil)
			},
		},
		{
			name: "limit order",
			config: Config{
				Symbol:       "BTCUSDT",
				Side:         "SELL",
				Quantity:     "0.002",
				Type:         "LIMIT",
				Price:        "45000",
				TimeInForce:  "GTC",
				PositionSide: "LONG",
				ReduceOnly:   "true",
			},
			expect: func(m *binance.MockIntegration) {
				m.EXPECT().PlaceFuturesOrder("BTCUSDT", "SELL", 0.002, "LIMIT", &price, "GTC", "LONG", true).
					Return(&binance.FuturesOrderResponse{OrderID: 12346, Symbol: "BTCUSDT", Status: "NEW"}, nil)
			},
		},
		{
			name:   "sets leverage",
			config: Config{Symbol: "BTCUSDT", Side: "BUY", Quantity: "0.001", Type: "MARKET", Leverage: "10"},
			expect: func(m *binance.MockIntegration) {
				m.EXPECT().SetLeverage("BTCUSDT", 10).Return(nil, nil)
				m.EXPECT().PlaceFuturesOrder("BTCUSDT", "BUY", 0.001, "MARKET", (*float64)(nil), "", "", false).
					Return(&binance.FuturesOrderResponse{OrderID: 12347, Symbol: "BTCUSDT", Status: "FILLED"}, nil)
			},
		},
		{
			name: "templated fields resolve against the request",
			config: Config{
				Symbol:       "{{ .symbol }}",
				Side:         "{{ .side }}",
				Quantity:     "{{ .quantity }}",
				Type:         "{{ .type }}",
				Price:        "{{ .price }}",
				TimeInForce:  "{{ .tif }}",
				PositionSide: "{{ .position_side }}",
				ReduceOnly:   "{{ .reduce_only }}",
				Leverage:     "{{ .leverage }}",
			},
			variables: map[string]any{
				"symbol":        "BTC\"USDT\"\nX",
				"side":          "SELL",
				"quantity":      "0.002",
				"type":          "LIMIT",
				"price":         "45000",
				"tif":           "IOC",
				"position_side": "SHORT",
				"reduce_only":   "true",
				"leverage":      "5",
			},
			expect: func(m *binance.MockIntegration) {
				m.EXPECT().SetLeverage("BTC\"USDT\"\nX", 5).Return(nil, nil)
				m.EXPECT().PlaceFuturesOrder("BTC\"USDT\"\nX", "SELL", 0.002, "LIMIT", &price, "IOC", "SHORT", true).
					Return(&binance.FuturesOrderResponse{OrderID: 12348, Status: "NEW"}, nil)
			},
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
			require.NoError(t, err)
		})
	}
}
