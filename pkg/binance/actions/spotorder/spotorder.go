package spotorder

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

	OrderTypeMarket          = "MARKET"
	OrderTypeLimit           = "LIMIT"
	OrderTypeStopLoss        = "STOP_LOSS"
	OrderTypeStopLossLimit   = "STOP_LOSS_LIMIT"
	OrderTypeTakeProfit      = "TAKE_PROFIT"
	OrderTypeTakeProfitLimit = "TAKE_PROFIT_LIMIT"

	TimeInForceGTC = "GTC"

	ActionType = "binance/spotorder"
)

type Config struct {
	Integration   string `json:"integration"`
	Symbol        string `json:"symbol"`
	Side          string `json:"side"`
	Quantity      string `json:"quantity,omitempty"`
	QuoteOrderQty string `json:"quote_order_qty,omitempty"`
	Type          string `json:"type"`
	Price         string `json:"price,omitempty"`
	StopPrice     string `json:"stop_price,omitempty"`
	TimeInForce   string `json:"time_in_force,omitempty"`
}

type Response struct {
	OrderID int64  `json:"orderId"`
	Symbol  string `json:"symbol"`
	Side    string `json:"side"`
	Type    string `json:"type"`
	Status  string `json:"status"`
}

type Executable struct {
	config Config
}

func (e *Executable) Type() string {
	return ActionType
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

	binanceIntegration, err := e.loadIntegration(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("error loading integration: %v", err)
	}

	if config.Symbol == "" {
		return nil, nil, errors.New("symbol is required")
	}

	if config.Side != SideBuy && config.Side != SideSell {
		return nil, nil, errors.New("side must be BUY or SELL")
	}

	if config.Quantity == "" && config.QuoteOrderQty == "" {
		return nil, nil, errors.New("quantity or quote_order_qty is required")
	}

	var quantity *float64
	if config.Quantity != "" {
		q, err := strconv.ParseFloat(config.Quantity, 64)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid quantity format: %v", err)
		}
		if q <= 0 {
			return nil, nil, errors.New("quantity must be greater than 0")
		}
		quantity = &q
	}

	var quoteOrderQty *float64
	if config.QuoteOrderQty != "" {
		q, err := strconv.ParseFloat(config.QuoteOrderQty, 64)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid quote_order_qty format: %v", err)
		}
		if q <= 0 {
			return nil, nil, errors.New("quote_order_qty must be greater than 0")
		}
		quoteOrderQty = &q
	}

	orderType := config.Type
	if orderType == "" {
		orderType = OrderTypeMarket
	}

	validOrderTypes := map[string]bool{
		OrderTypeMarket:          true,
		OrderTypeLimit:           true,
		OrderTypeStopLoss:        true,
		OrderTypeStopLossLimit:   true,
		OrderTypeTakeProfit:      true,
		OrderTypeTakeProfitLimit: true,
	}
	if !validOrderTypes[orderType] {
		return nil, nil, errors.New("type must be MARKET, LIMIT, STOP_LOSS, STOP_LOSS_LIMIT, TAKE_PROFIT, or TAKE_PROFIT_LIMIT")
	}

	// quoteOrderQty is only valid for MARKET orders
	if quoteOrderQty != nil && orderType != OrderTypeMarket {
		return nil, nil, errors.New("quote_order_qty is only valid for MARKET orders")
	}

	var price *float64
	requiresPrice := orderType == OrderTypeLimit || orderType == OrderTypeStopLossLimit || orderType == OrderTypeTakeProfitLimit
	if requiresPrice {
		if config.Price == "" {
			return nil, nil, fmt.Errorf("price is required for %s orders", orderType)
		}
		p, err := strconv.ParseFloat(config.Price, 64)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid price format: %v", err)
		}
		if p <= 0 {
			return nil, nil, fmt.Errorf("price must be greater than 0 for %s orders", orderType)
		}
		price = &p

		if config.TimeInForce == "" {
			config.TimeInForce = TimeInForceGTC
		}
	}

	var stopPrice *float64
	requiresStopPrice := orderType == OrderTypeStopLoss || orderType == OrderTypeStopLossLimit || orderType == OrderTypeTakeProfit || orderType == OrderTypeTakeProfitLimit
	if requiresStopPrice {
		if config.StopPrice == "" {
			return nil, nil, fmt.Errorf("stop_price is required for %s orders", orderType)
		}
		sp, err := strconv.ParseFloat(config.StopPrice, 64)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid stop_price format: %v", err)
		}
		if sp <= 0 {
			return nil, nil, fmt.Errorf("stop_price must be greater than 0 for %s orders", orderType)
		}
		stopPrice = &sp
	}

	orderResponse, err := binanceIntegration.PlaceSpotOrder(
		config.Symbol,
		config.Side,
		quantity,
		quoteOrderQty,
		orderType,
		price,
		stopPrice,
		config.TimeInForce,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to place spot order: %v", err)
	}

	return Response{
		OrderID: orderResponse.OrderID,
		Symbol:  orderResponse.Symbol,
		Side:    orderResponse.Side,
		Type:    orderResponse.Type,
		Status:  orderResponse.Status,
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
