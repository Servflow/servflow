package futuresorder

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	"github.com/Servflow/servflow/pkg/engine/requestctx"

	"github.com/Servflow/servflow/pkg/binance"
)

const (
	SideBuy  = "BUY"
	SideSell = "SELL"

	OrderTypeMarket           = "MARKET"
	OrderTypeLimit            = "LIMIT"
	OrderTypeStop             = "STOP"
	OrderTypeStopMarket       = "STOP_MARKET"
	OrderTypeTakeProfit       = "TAKE_PROFIT"
	OrderTypeTakeProfitMarket = "TAKE_PROFIT_MARKET"

	PositionSideBoth  = "BOTH"
	PositionSideLong  = "LONG"
	PositionSideShort = "SHORT"

	TimeInForceGTC = "GTC"

	ActionType = "binance/futuresorder"
)

type Config struct {
	Integration  string `json:"integration"`
	Symbol       string `json:"symbol"`
	Side         string `json:"side"`
	Quantity     string `json:"quantity"`
	Type         string `json:"type"`
	Price        string `json:"price,omitempty"`
	TimeInForce  string `json:"time_in_force,omitempty"`
	PositionSide string `json:"position_side,omitempty"`
	ReduceOnly   string `json:"reduce_only,omitempty"`
	Leverage     string `json:"leverage,omitempty"`
}

// Response represents the response for futures orders
type Response struct {
	OrderID      int64  `json:"orderId"`
	Symbol       string `json:"symbol"`
	Side         string `json:"side"`
	Type         string `json:"type"`
	ExecutedQty  string `json:"executedQty"`
	Status       string `json:"status"`
	PositionSide string `json:"positionSide"`
	ReduceOnly   bool   `json:"reduceOnly"`
}

type Executable struct {
	config      Config
	integration binance.Integration // optional, for testing
}

func (e *Executable) Type() string {
	return ActionType
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
	binanceIntegration, err := e.loadIntegration(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("error loading integration: %v", err)
	}

	var config Config
	if modifiedConfig != "" {
		if err := json.Unmarshal([]byte(modifiedConfig), &config); err != nil {
			return nil, nil, fmt.Errorf("error parsing config: %v", err)
		}
	} else {
		config = e.config
	}

	// Validate required fields
	if config.Symbol == "" {
		return nil, nil, errors.New("symbol is required")
	}

	if config.Side != SideBuy && config.Side != SideSell {
		return nil, nil, errors.New("side must be BUY or SELL")
	}

	if config.Quantity == "" {
		return nil, nil, errors.New("quantity is required")
	}

	// Convert quantity string to float64
	quantity, err := strconv.ParseFloat(config.Quantity, 64)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid quantity format: %v", err)
	}
	if quantity <= 0 {
		return nil, nil, errors.New("quantity must be greater than 0")
	}

	// Default to MARKET if type not specified
	orderType := config.Type
	if orderType == "" {
		orderType = OrderTypeMarket
	}

	// Validate order type
	if orderType != OrderTypeMarket && orderType != OrderTypeLimit && orderType != OrderTypeStop && orderType != OrderTypeStopMarket && orderType != OrderTypeTakeProfit && orderType != OrderTypeTakeProfitMarket {
		return nil, nil, errors.New("type must be MARKET, LIMIT, STOP, STOP_MARKET, TAKE_PROFIT, or TAKE_PROFIT_MARKET")
	}

	var price *float64

	// Handle LIMIT order requirements
	if orderType == OrderTypeLimit || orderType == OrderTypeStop || orderType == OrderTypeTakeProfit {
		if config.Price == "" {
			return nil, nil, fmt.Errorf("price is required for %s orders", orderType)
		}

		priceVal, err := strconv.ParseFloat(config.Price, 64)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid price format: %v", err)
		}
		if priceVal <= 0 {
			return nil, nil, fmt.Errorf("price must be greater than 0 for %s orders", orderType)
		}
		price = &priceVal

		if config.TimeInForce == "" {
			config.TimeInForce = TimeInForceGTC // Good Till Canceled as default
		}
	}

	// Validate position side (optional, defaults to BOTH for hedge mode)
	if config.PositionSide != "" && config.PositionSide != PositionSideBoth && config.PositionSide != PositionSideLong && config.PositionSide != PositionSideShort {
		return nil, nil, errors.New("position_side must be BOTH, LONG, or SHORT")
	}

	// Handle reduce only flag
	var reduceOnly bool
	if config.ReduceOnly != "" {
		reduceOnly, err = strconv.ParseBool(config.ReduceOnly)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid reduce_only format: %v", err)
		}
	}

	if config.Leverage != "" {
		leverage, err := strconv.ParseInt(config.Leverage, 10, 64)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid leverage format: %v", err)
		}
		if leverage <= 0 {
			return nil, nil, fmt.Errorf("leverage must be greater than 0")
		}

		_, err = binanceIntegration.SetLeverage(config.Symbol, int(leverage))
		if err != nil {
			return nil, nil, fmt.Errorf("error setting leverage: %v", err)
		}
	}

	orderResponse, err := binanceIntegration.PlaceFuturesOrder(config.Symbol, config.Side, quantity, orderType, price, config.TimeInForce, config.PositionSide, reduceOnly)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to place futures order: %v", err)
	}

	// Return simplified response
	return Response{
		OrderID:      orderResponse.OrderID,
		Symbol:       orderResponse.Symbol,
		Side:         orderResponse.Side,
		Type:         orderResponse.Type,
		ExecutedQty:  orderResponse.ExecutedQty,
		Status:       orderResponse.Status,
		PositionSide: orderResponse.PositionSide,
		ReduceOnly:   orderResponse.ReduceOnly,
	}, nil, nil
}

func NewExecutable(cfg Config) (*Executable, error) {
	if cfg.Integration == "" {
		return nil, errors.New("integration is required")
	}

	return &Executable{
		config: cfg,
	}, nil
}
