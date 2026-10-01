package spotorder

import (
	"context"
	"testing"

	"github.com/Servflow/servflow/pkg/engine/requestctx"
	"github.com/Servflow/servflow/pkg/engine/requestctx/requestctxtest"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/Servflow/servflow/pkg/binance"
)

// rc is the request every test here runs in; setupMockIntegration configures
// its integrations.
var rc = requestctxtest.New()

func setupMockIntegration(t *testing.T, ctrl *gomock.Controller, integrationID string) *binance.MockIntegration {
	t.Helper()
	mockIntegration := binance.NewMockIntegration(ctrl)
	rc.SetIntegration(integrationID, mockIntegration)
	return mockIntegration
}

func TestExecute_WithQuantity(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	integrationID := "test-spot-qty"
	mock := setupMockIntegration(t, ctrl, integrationID)

	qty := 0.001
	price := 45000.0
	stopPrice := 44000.0

	mock.EXPECT().PlaceSpotOrder(
		"BTCUSDT",
		SideBuy,
		&qty,
		(*float64)(nil),
		OrderTypeStopLossLimit,
		&price,
		&stopPrice,
		TimeInForceGTC,
	).Return(&binance.OrderResponse{OrderID: 123}, nil)

	exec := &Executable{config: Config{
		Integration: integrationID,
		Symbol:      "BTCUSDT",
		Side:        SideBuy,
		Quantity:    "0.001",
		Type:        OrderTypeStopLossLimit,
		Price:       "45000",
		StopPrice:   "44000",
		TimeInForce: TimeInForceGTC,
	}}

	_, _, err := exec.Execute(requestctx.With(context.Background(), rc), "")
	require.NoError(t, err)
}

func TestExecute_WithQuoteOrderQty(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	integrationID := "test-spot-quote"
	mock := setupMockIntegration(t, ctrl, integrationID)

	quoteQty := 100.0

	mock.EXPECT().PlaceSpotOrder(
		"BTCUSDT",
		SideBuy,
		(*float64)(nil),
		&quoteQty,
		OrderTypeMarket,
		(*float64)(nil),
		(*float64)(nil),
		"",
	).Return(&binance.OrderResponse{OrderID: 123}, nil)

	exec := &Executable{config: Config{
		Integration:   integrationID,
		Symbol:        "BTCUSDT",
		Side:          SideBuy,
		QuoteOrderQty: "100",
		Type:          OrderTypeMarket,
	}}

	_, _, err := exec.Execute(requestctx.With(context.Background(), rc), "")
	require.NoError(t, err)
}
