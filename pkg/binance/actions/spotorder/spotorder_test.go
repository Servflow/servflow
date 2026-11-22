package spotorder

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/Servflow/servflow/pkg/binance"
	"github.com/Servflow/servflow/pkg/engine/requestctx"
	"github.com/Servflow/servflow/pkg/engine/requestctx/requestctxtest"
)

func ptr(f float64) *float64 { return &f }

func TestExecute(t *testing.T) {
	tests := []struct {
		name      string
		config    Config
		variables map[string]any
		expect    func(m *binance.MockIntegration)
	}{
		{
			name: "stop loss limit order with quantity",
			config: Config{
				Symbol:      "BTCUSDT",
				Side:        SideBuy,
				Quantity:    "0.001",
				Type:        OrderTypeStopLossLimit,
				Price:       "45000",
				StopPrice:   "44000",
				TimeInForce: TimeInForceGTC,
			},
			expect: func(m *binance.MockIntegration) {
				m.EXPECT().PlaceSpotOrder("BTCUSDT", SideBuy, ptr(0.001), (*float64)(nil), OrderTypeStopLossLimit, ptr(45000), ptr(44000), TimeInForceGTC).
					Return(&binance.OrderResponse{OrderID: 123}, nil)
			},
		},
		{
			name: "market order with quote order quantity",
			config: Config{
				Symbol:        "BTCUSDT",
				Side:          SideBuy,
				QuoteOrderQty: "100",
				Type:          OrderTypeMarket,
			},
			expect: func(m *binance.MockIntegration) {
				m.EXPECT().PlaceSpotOrder("BTCUSDT", SideBuy, (*float64)(nil), ptr(100), OrderTypeMarket, (*float64)(nil), (*float64)(nil), "").
					Return(&binance.OrderResponse{OrderID: 123}, nil)
			},
		},
		{
			name: "templated fields resolve against the request",
			config: Config{
				Symbol:      "{{ .symbol }}",
				Side:        "{{ .side }}",
				Quantity:    "{{ .quantity }}",
				Type:        "{{ .type }}",
				Price:       "{{ .price }}",
				TimeInForce: "{{ .tif }}",
			},
			variables: map[string]any{
				"symbol":   "BTC\"USDT\"\nX",
				"side":     SideSell,
				"quantity": "0.5",
				"type":     OrderTypeLimit,
				"price":    "45000",
				"tif":      "IOC",
			},
			expect: func(m *binance.MockIntegration) {
				m.EXPECT().PlaceSpotOrder("BTC\"USDT\"\nX", SideSell, ptr(0.5), (*float64)(nil), OrderTypeLimit, ptr(45000), (*float64)(nil), "IOC").
					Return(&binance.OrderResponse{OrderID: 123}, nil)
			},
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
			exec := &Executable{config: cfg}
			_, _, err := exec.Execute(requestctx.With(context.Background(), rc))
			require.NoError(t, err)
		})
	}
}
