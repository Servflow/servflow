package pricedifference

import (
	"context"
	"testing"

	"github.com/Servflow/servflow/pkg/engine/requestctx"
	"github.com/Servflow/servflow/pkg/engine/requestctx/requestctxtest"
	"github.com/stretchr/testify/assert"
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

func TestExecute_SingleSymbol(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	integrationID := "test-single"
	mockIntegration := setupMockIntegration(t, ctrl, integrationID)

	mockIntegration.EXPECT().GetCurrentPrice("BTCUSDT").Return(50000.0, nil)
	mockIntegration.EXPECT().GetHistoricalPrice("BTCUSDT", "1h", 24).Return([]*binance.KlineData{{Close: "48000.00"}}, nil)

	executable := &Executable{config: Config{Integration: integrationID, Symbol: "BTCUSDT", Interval: "1h", Period: "24"}}
	_, _, err := executable.Execute(requestctx.With(context.Background(), rc), "")
	require.NoError(t, err)
}

func TestExecute_MultipleSymbols(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	integrationID := "test-multi"
	mockIntegration := setupMockIntegration(t, ctrl, integrationID)

	mockIntegration.EXPECT().GetCurrentPrice("BTCUSDT").Return(50000.0, nil)
	mockIntegration.EXPECT().GetHistoricalPrice("BTCUSDT", "1h", 24).Return([]*binance.KlineData{{Close: "48000.00"}}, nil)
	mockIntegration.EXPECT().GetCurrentPrice("ETHUSDT").Return(3000.0, nil)
	mockIntegration.EXPECT().GetHistoricalPrice("ETHUSDT", "1h", 24).Return([]*binance.KlineData{{Close: "2900.00"}}, nil)

	executable := &Executable{config: Config{Integration: integrationID, Symbol: "BTCUSDT,ETHUSDT", Interval: "1h", Period: "24"}}
	_, _, err := executable.Execute(requestctx.With(context.Background(), rc), "")
	require.NoError(t, err)
}

func TestExecute_APIError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	integrationID := "test-error"
	mockIntegration := setupMockIntegration(t, ctrl, integrationID)

	mockIntegration.EXPECT().GetCurrentPrice("BTCUSDT").Return(0.0, assert.AnError)

	executable := &Executable{config: Config{Integration: integrationID, Symbol: "BTCUSDT", Interval: "1h", Period: "24"}}
	_, _, err := executable.Execute(requestctx.With(context.Background(), rc), "")
	require.Error(t, err)
}
