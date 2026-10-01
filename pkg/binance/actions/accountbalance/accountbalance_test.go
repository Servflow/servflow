package accountbalance

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

func TestExecute_Spot(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	integrationID := "test-spot"
	mock := setupMockIntegration(t, ctrl, integrationID)

	mock.EXPECT().GetAccountBalance().Return(&binance.AccountBalance{
		Balances: []*binance.Balance{{Asset: "BTC", Free: "1.0"}},
	}, nil)

	exec := &Executable{config: Config{Integration: integrationID, Symbol: "BTC"}}
	_, _, err := exec.Execute(requestctx.With(context.Background(), rc), "")
	require.NoError(t, err)
}

func TestExecute_Futures(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	integrationID := "test-futures"
	mock := setupMockIntegration(t, ctrl, integrationID)

	mock.EXPECT().GetFuturesAccountInfo().Return(&binance.FuturesAccountInfo{
		Assets: []*binance.FuturesAccountAsset{{Asset: "USDT", WalletBalance: "500.0"}},
	}, nil)

	exec := &Executable{config: Config{Integration: integrationID, Symbol: "USDT", Futures: true}}
	_, _, err := exec.Execute(requestctx.With(context.Background(), rc), "")
	require.NoError(t, err)
}

func TestExecute_APIError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	integrationID := "test-error"
	mock := setupMockIntegration(t, ctrl, integrationID)

	mock.EXPECT().GetAccountBalance().Return(nil, assert.AnError)

	exec := &Executable{config: Config{Integration: integrationID, Symbol: "BTC"}}
	_, _, err := exec.Execute(requestctx.With(context.Background(), rc), "")
	require.Error(t, err)
}
