package tradeinfo

import (
	"context"
	"testing"

	"github.com/Servflow/servflow/pkg/binance"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestExecute_SpotOpenOrders(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockIntegration := binance.NewMockIntegration(ctrl)

	mockIntegration.EXPECT().GetOpenOrders("BTCUSDT").Return(&binance.OpenOrdersResponse{}, nil)

	executable := &Executable{
		config:      Config{Integration: "test", Symbol: "BTCUSDT", Futures: false},
		integration: mockIntegration,
	}

	_, _, err := executable.Execute(context.Background(), "")
	require.NoError(t, err)
}

func TestExecute_SpotOrderInfo(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockIntegration := binance.NewMockIntegration(ctrl)

	mockIntegration.EXPECT().GetOrderInfo("BTCUSDT", int64(12345)).Return(&binance.OrderInfo{}, nil)

	executable := &Executable{
		config:      Config{Integration: "test", Symbol: "BTCUSDT", OrderID: "12345", Futures: false},
		integration: mockIntegration,
	}

	_, _, err := executable.Execute(context.Background(), "")
	require.NoError(t, err)
}

func TestExecute_FuturesOpenOrders(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockIntegration := binance.NewMockIntegration(ctrl)

	mockIntegration.EXPECT().GetFuturesOpenOrders("BTCUSDT").Return(&binance.FuturesOpenOrdersResponse{}, nil)

	executable := &Executable{
		config:      Config{Integration: "test", Symbol: "BTCUSDT", Futures: true},
		integration: mockIntegration,
	}

	_, _, err := executable.Execute(context.Background(), "")
	require.NoError(t, err)
}

func TestExecute_FuturesPositions(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockIntegration := binance.NewMockIntegration(ctrl)

	mockIntegration.EXPECT().GetAllPositions("BTCUSDT").Return(&binance.PositionsResponse{}, nil)

	executable := &Executable{
		config:      Config{Integration: "test", Symbol: "BTCUSDT", Futures: true, InfoType: InfoTypePositions},
		integration: mockIntegration,
	}

	_, _, err := executable.Execute(context.Background(), "")
	require.NoError(t, err)
}

func TestExecute_APIError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockIntegration := binance.NewMockIntegration(ctrl)

	mockIntegration.EXPECT().GetOpenOrders("BTCUSDT").Return(nil, assert.AnError)

	executable := &Executable{
		config:      Config{Integration: "test", Symbol: "BTCUSDT", Futures: false},
		integration: mockIntegration,
	}

	_, _, err := executable.Execute(context.Background(), "")
	require.Error(t, err)
}
