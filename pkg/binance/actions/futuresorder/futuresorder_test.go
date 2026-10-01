package futuresorder

import (
	"context"
	"testing"

	"github.com/Servflow/servflow/pkg/binance"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestExecute_PlacesOrder(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockIntegration := binance.NewMockIntegration(ctrl)
	mockIntegration.EXPECT().PlaceFuturesOrder(
		"BTCUSDT",
		"BUY",
		0.001,
		"MARKET",
		(*float64)(nil),
		"",
		"",
		false,
	).Return(&binance.FuturesOrderResponse{
		OrderID: 12345,
		Symbol:  "BTCUSDT",
		Status:  "FILLED",
	}, nil)

	executable := &Executable{
		config: Config{
			Integration: "test",
			Symbol:      "BTCUSDT",
			Side:        "BUY",
			Quantity:    "0.001",
			Type:        "MARKET",
		},
		integration: mockIntegration,
	}

	_, _, err := executable.Execute(context.Background(), "")
	require.NoError(t, err)
}

func TestExecute_LimitOrder(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockIntegration := binance.NewMockIntegration(ctrl)
	price := 45000.0
	mockIntegration.EXPECT().PlaceFuturesOrder(
		"BTCUSDT",
		"SELL",
		0.002,
		"LIMIT",
		&price,
		"GTC",
		"LONG",
		true,
	).Return(&binance.FuturesOrderResponse{
		OrderID: 12346,
		Symbol:  "BTCUSDT",
		Status:  "NEW",
	}, nil)

	executable := &Executable{
		config: Config{
			Integration:  "test",
			Symbol:       "BTCUSDT",
			Side:         "SELL",
			Quantity:     "0.002",
			Type:         "LIMIT",
			Price:        "45000",
			TimeInForce:  "GTC",
			PositionSide: "LONG",
			ReduceOnly:   "true",
		},
		integration: mockIntegration,
	}

	_, _, err := executable.Execute(context.Background(), "")
	require.NoError(t, err)
}

func TestExecute_SetsLeverage(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockIntegration := binance.NewMockIntegration(ctrl)
	mockIntegration.EXPECT().SetLeverage("BTCUSDT", 10).Return(nil, nil)
	mockIntegration.EXPECT().PlaceFuturesOrder(
		"BTCUSDT",
		"BUY",
		0.001,
		"MARKET",
		(*float64)(nil),
		"",
		"",
		false,
	).Return(&binance.FuturesOrderResponse{
		OrderID: 12347,
		Symbol:  "BTCUSDT",
		Status:  "FILLED",
	}, nil)

	executable := &Executable{
		config: Config{
			Integration: "test",
			Symbol:      "BTCUSDT",
			Side:        "BUY",
			Quantity:    "0.001",
			Type:        "MARKET",
			Leverage:    "10",
		},
		integration: mockIntegration,
	}

	_, _, err := executable.Execute(context.Background(), "")
	require.NoError(t, err)
}
