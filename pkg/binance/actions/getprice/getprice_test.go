package getprice

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/Servflow/servflow/pkg/binance"
	"github.com/Servflow/servflow/pkg/engine/requestctx/requestctxtest"
)

func TestExecute_CurrentPrice(t *testing.T) {
	ctrl := gomock.NewController(t)
	mock := binance.NewMockIntegration(ctrl)

	mock.EXPECT().GetCurrentPrice("BTCUSDT").Return(50000.0, nil)

	e := &Executable{config: Config{Symbol: "BTCUSDT", PriceType: "current"}, integration: mock}
	_, _, err := e.Execute(requestctxtest.NewContext(), "")
	require.NoError(t, err)
}

func TestExecute_24hr(t *testing.T) {
	ctrl := gomock.NewController(t)
	mock := binance.NewMockIntegration(ctrl)

	mock.EXPECT().Get24HrTicker("ETHUSDT").Return(&binance.TickerStats{}, nil)

	e := &Executable{config: Config{Symbol: "ETHUSDT", PriceType: "24hr"}, integration: mock}
	_, _, err := e.Execute(requestctxtest.NewContext(), "")
	require.NoError(t, err)
}

func TestExecute_EmptySymbol(t *testing.T) {
	e := &Executable{config: Config{Symbol: "", PriceType: "current"}}
	_, _, err := e.Execute(requestctxtest.NewContext(), "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "symbol is required")
}

func TestExecute_InvalidPriceType(t *testing.T) {
	ctrl := gomock.NewController(t)
	mock := binance.NewMockIntegration(ctrl)

	e := &Executable{config: Config{Symbol: "BTCUSDT", PriceType: "invalid"}, integration: mock}
	_, _, err := e.Execute(requestctxtest.NewContext(), "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid price_type")
}

func TestExecute_APIError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mock := binance.NewMockIntegration(ctrl)

	mock.EXPECT().GetCurrentPrice("BTCUSDT").Return(0.0, assert.AnError)

	e := &Executable{config: Config{Symbol: "BTCUSDT", PriceType: "current"}, integration: mock}
	_, _, err := e.Execute(requestctxtest.NewContext(), "")
	require.Error(t, err)
}
