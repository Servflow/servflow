package accountbalance

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Servflow/servflow/pkg/engine/requestctx"

	"github.com/Servflow/servflow/pkg/binance"
)

type Balance struct {
	Symbol  string `json:"symbol"`
	Balance string `json:"balance"`
	Time    int64  `json:"time"`
}

type Config struct {
	Integration string `json:"integration"`
	Symbol      string `json:"symbol,omitempty"`
	Futures     bool   `json:"futures,omitempty"`
}

type Executable struct {
	config Config
}

func (e *Executable) Type() string {
	return "binance/accountbalance"
}

func (e *Executable) Config() string {
	configBytes, _ := json.Marshal(e.config)
	return string(configBytes)
}

func (e *Executable) loadIntegration(ctx context.Context) (binance.Integration, error) {
	i, err := requestctx.GetIntegration(ctx, e.config.Integration)
	if err != nil {
		return nil, err
	}
	binanceIntegration, ok := i.(binance.Integration)
	if !ok {
		return nil, fmt.Errorf("integration is not a binance.Integration")
	}
	return binanceIntegration, nil
}

func (e *Executable) Execute(ctx context.Context, modifiedConfig string) (interface{}, map[string]string, error) {
	config := e.config
	if modifiedConfig != "" {
		if err := json.Unmarshal([]byte(modifiedConfig), &config); err != nil {
			return nil, nil, fmt.Errorf("error parsing config: %v", err)
		}
	}

	binanceIntegration, err := e.loadIntegration(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("error loading integration: %v", err)
	}

	symbols := parseSymbols(config.Symbol)

	if config.Futures {
		resp, err := e.getFuturesBalances(binanceIntegration, symbols)
		return resp, nil, err
	}

	var fields = make(map[string]string)
	resp, err := e.getSpotBalances(binanceIntegration, symbols)
	if err != nil {
		fields["error"] = err.Error()
	}
	return resp, fields, err
}

func (e *Executable) getSpotBalances(binanceIntegration binance.Integration, symbols []string) (interface{}, error) {
	accountBalance, err := binanceIntegration.GetAccountBalance()
	if err != nil {
		return nil, fmt.Errorf("error getting account balance: %v", err)
	}

	now := time.Now().Unix()
	balanceMap := make(map[string]string, len(accountBalance.Balances))
	for _, b := range accountBalance.Balances {
		balanceMap[b.Asset] = b.Free
	}

	if len(symbols) == 0 {
		return buildAllBalances(accountBalance.Balances, now), nil
	}

	return buildFilteredBalances(symbols, balanceMap, now), nil
}

func (e *Executable) getFuturesBalances(binanceIntegration binance.Integration, symbols []string) (interface{}, error) {
	futuresInfo, err := binanceIntegration.GetFuturesAccountInfo()
	if err != nil {
		return nil, fmt.Errorf("error getting futures account balance: %v", err)
	}

	now := time.Now().Unix()
	balanceMap := make(map[string]string, len(futuresInfo.Assets))
	for _, a := range futuresInfo.Assets {
		balanceMap[a.Asset] = a.WalletBalance
	}

	if len(symbols) == 0 {
		return buildAllFuturesBalances(futuresInfo.Assets, now), nil
	}

	return buildFilteredBalances(symbols, balanceMap, now), nil
}

func parseSymbols(symbolStr string) []string {
	if symbolStr == "" {
		return nil
	}

	parts := strings.Split(symbolStr, ",")
	var symbols []string
	for _, p := range parts {
		if s := strings.TrimSpace(p); s != "" {
			symbols = append(symbols, s)
		}
	}
	return symbols
}

func buildAllBalances(balances []*binance.Balance, timestamp int64) []Balance {
	result := make([]Balance, 0, len(balances))
	for _, b := range balances {
		result = append(result, Balance{
			Symbol:  b.Asset,
			Balance: b.Free,
			Time:    timestamp,
		})
	}
	return result
}

func buildAllFuturesBalances(assets []*binance.FuturesAccountAsset, timestamp int64) []Balance {
	result := make([]Balance, 0, len(assets))
	for _, a := range assets {
		result = append(result, Balance{
			Symbol:  a.Asset,
			Balance: a.WalletBalance,
			Time:    timestamp,
		})
	}
	return result
}

func buildFilteredBalances(symbols []string, balanceMap map[string]string, timestamp int64) interface{} {
	results := make([]Balance, 0, len(symbols))
	for _, symbol := range symbols {
		balance := balanceMap[symbol]
		if balance == "" {
			balance = "0"
		}
		results = append(results, Balance{
			Symbol:  symbol,
			Balance: balance,
			Time:    timestamp,
		})
	}

	if len(results) == 1 {
		return results[0]
	}
	return results
}

func NewExecutable(cfg Config) (*Executable, error) {
	if cfg.Integration == "" {
		return nil, fmt.Errorf("integration is required")
	}

	return &Executable{
		config: cfg,
	}, nil
}
