package accountbalance

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
		want      any
		wantErr   bool
	}{
		{
			name:   "spot",
			config: Config{Symbol: "BTC"},
			expect: func(m *binance.MockIntegration) {
				m.EXPECT().GetAccountBalance().Return(&binance.AccountBalance{
					Balances: []*binance.Balance{{Asset: "BTC", Free: "1.0"}},
				}, nil)
			},
			want: Balance{Symbol: "BTC", Balance: "1.0"},
		},
		{
			name:   "futures",
			config: Config{Symbol: "USDT", Futures: true},
			expect: func(m *binance.MockIntegration) {
				m.EXPECT().GetFuturesAccountInfo().Return(&binance.FuturesAccountInfo{
					Assets: []*binance.FuturesAccountAsset{{Asset: "USDT", WalletBalance: "500.0"}},
				}, nil)
			},
			want: Balance{Symbol: "USDT", Balance: "500.0"},
		},
		{
			name:      "templated symbol resolves against the request",
			config:    Config{Symbol: "{{ .symbol }}"},
			variables: map[string]any{"symbol": "B\"T\"C"},
			expect: func(m *binance.MockIntegration) {
				m.EXPECT().GetAccountBalance().Return(&binance.AccountBalance{
					Balances: []*binance.Balance{{Asset: "B\"T\"C", Free: "2.0"}},
				}, nil)
			},
			want: Balance{Symbol: "B\"T\"C", Balance: "2.0"},
		},
		{
			name:   "api error",
			config: Config{Symbol: "BTC"},
			expect: func(m *binance.MockIntegration) {
				m.EXPECT().GetAccountBalance().Return(nil, assert.AnError)
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
			exec := &Executable{config: cfg}
			got, _, err := exec.Execute(requestctx.With(context.Background(), rc))
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			balance, ok := got.(Balance)
			require.True(t, ok)
			balance.Time = 0
			assert.Equal(t, tt.want, balance)
		})
	}
}
