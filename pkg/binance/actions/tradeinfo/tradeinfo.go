package tradeinfo

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/Servflow/servflow/pkg/engine/requestctx"

	"github.com/Servflow/servflow/pkg/binance"
)

type Config struct {
	Integration string `json:"integration"`
	Symbol      string `json:"symbol"`
	OrderID     string `json:"orderID"`
	Futures     bool   `json:"futures,omitempty"`
	InfoType    string `json:"infoType,omitempty"` // "orders" (default) or "positions"
}

const (
	InfoTypeOrders    = "orders"
	InfoTypePositions = "positions"
)

type Executable struct {
	config      Config
	integration binance.Integration // For testing; if nil, loadIntegration is used
}

func (e *Executable) Type() string {
	return "binance/tradeinfo"
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

func (e *Executable) Execute(ctx context.Context) (any, map[string]string, error) {
	rc, err := requestctx.FromContextOrError(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("error getting request context: %w", err)
	}
	resolved, err := rc.ResolveBatch(ctx, e.config.Symbol, e.config.OrderID, e.config.InfoType)
	if err != nil {
		return nil, nil, fmt.Errorf("error resolving config: %w", err)
	}
	config := e.config
	config.Symbol = resolved[0]
	config.OrderID = resolved[1]
	config.InfoType = resolved[2]

	binanceIntegration, err := e.loadIntegration(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("error loading integration: %v", err)
	}

	if config.InfoType == "" {
		config.InfoType = InfoTypeOrders
	}

	// Validate required fields
	// For futures orders, symbol is optional (empty means get all open orders)
	// For spot trading and futures positions, symbol is still required
	if config.Symbol == "" && (!config.Futures || config.InfoType == InfoTypePositions) {
		return nil, nil, errors.New("symbol is required")
	}
	if config.Futures {
		resp, err := e.getFuturesInfo(config, binanceIntegration)
		return resp, nil, err
	} else {
		resp, err := e.getSpotInfo(config, binanceIntegration)
		return resp, nil, err
	}
}

func (e *Executable) getSpotInfo(config Config, binanceIntegration binance.Integration) (interface{}, error) {
	if config.OrderID == "" {
		if config.InfoType != InfoTypeOrders {
			return nil, fmt.Errorf("invalid infoType '%s' for spot trading, only 'orders' is supported", config.InfoType)
		}
		openOrders, err := binanceIntegration.GetOpenOrders(config.Symbol)
		if err != nil {
			return nil, fmt.Errorf("error getting open orders: %v", err)
		}
		return openOrders, nil
	} else {
		// Convert orderID string to int64
		orderID, err := strconv.ParseInt(config.OrderID, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid orderID format: %v", err)
		}
		if orderID <= 0 {
			return nil, errors.New("orderID must be greater than 0")
		}
		orderInfo, err := binanceIntegration.GetOrderInfo(config.Symbol, orderID)
		if err != nil {
			return nil, fmt.Errorf("error getting order info: %v", err)
		}
		return orderInfo, nil
	}
}

func (e *Executable) getFuturesInfo(config Config, binanceIntegration binance.Integration) (interface{}, error) {
	if config.OrderID != "" {
		return nil, errors.New("getting futures specific order info not supported")
	}
	if config.Symbol == "" && config.InfoType == InfoTypePositions {
		return nil, errors.New("symbol is required for getting futures positions")
	}

	switch config.InfoType {
	case InfoTypePositions:
		positionsResponse, err := binanceIntegration.GetAllPositions(config.Symbol)
		if err != nil {
			return nil, fmt.Errorf("error getting positions: %v", err)
		}
		return positionsResponse, nil
	case InfoTypeOrders:
		openOrders, err := binanceIntegration.GetFuturesOpenOrders(config.Symbol)
		if err != nil {
			return nil, fmt.Errorf("error getting futures open orders: %v", err)
		}
		return openOrders, nil
	default:
		return nil, fmt.Errorf("invalid infoType '%s' for futures, must be 'orders' or 'positions'", config.InfoType)
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
