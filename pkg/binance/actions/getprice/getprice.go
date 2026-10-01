package getprice

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/Servflow/servflow/pkg/engine/requestctx"

	"github.com/Servflow/servflow/pkg/binance"
)

type Config struct {
	Integration string `json:"integration"`
	Symbol      string `json:"symbol"`
	PriceType   string `json:"price_type"`
}

// CurrentPriceResponse represents the response for current price queries
type CurrentPriceResponse struct {
	Symbol string  `json:"symbol"`
	Price  float64 `json:"price"`
	Type   string  `json:"type"`
}

type Executable struct {
	config      Config
	integration binance.Integration // optional, for testing
}

func (e *Executable) Type() string {
	return "binance/getprice"
}

func (e *Executable) Config() string {
	configBytes, _ := json.Marshal(e.config)
	return string(configBytes)
}

func (e *Executable) loadIntegration(ctx context.Context) (binance.Integration, error) {
	if e.integration != nil {
		return e.integration, nil
	}
	i, err := requestctx.GetIntegration(ctx, e.config.Integration)
	if err != nil {
		return nil, err
	}
	binanceIntegration, ok := i.(binance.Integration)
	if !ok {
		return nil, errors.New("integration is not a binance.Integration")
	}
	return binanceIntegration, nil
}

func (e *Executable) Execute(ctx context.Context, modifiedConfig string) (interface{}, map[string]string, error) {
	var config Config
	if modifiedConfig != "" {
		if err := json.Unmarshal([]byte(modifiedConfig), &config); err != nil {
			return nil, nil, fmt.Errorf("error parsing config: %v", err)
		}
	} else {
		config = e.config
	}

	if config.Symbol == "" {
		return nil, nil, errors.New("symbol is required")
	}

	binanceIntegration, err := e.loadIntegration(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("error loading integration: %v", err)
	}

	switch config.PriceType {
	case "current":
		price, err := binanceIntegration.GetCurrentPrice(config.Symbol)
		if err != nil {
			return nil, nil, err
		}
		return CurrentPriceResponse{
			Symbol: config.Symbol,
			Price:  price,
			Type:   "current",
		}, nil, nil

	case "24hr":
		ticker, err := binanceIntegration.Get24HrTicker(config.Symbol)
		if err != nil {
			return nil, nil, err
		}
		return ticker, nil, nil

	default:
		return nil, nil, errors.New("invalid price_type, must be 'current' or '24hr'")
	}
}

func NewExecutable(cfg Config) (*Executable, error) {
	if cfg.Integration == "" {
		return nil, errors.New("integration is required")
	}

	return &Executable{
		config: cfg,
	}, nil
}
