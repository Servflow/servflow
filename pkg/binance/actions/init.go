package actions

import (
	"encoding/json"
	"fmt"

	"github.com/Servflow/servflow/pkg/engine/actions"

	"github.com/Servflow/servflow/pkg/binance/actions/accountbalance"
	"github.com/Servflow/servflow/pkg/binance/actions/futuresorder"
	"github.com/Servflow/servflow/pkg/binance/actions/getprice"
	"github.com/Servflow/servflow/pkg/binance/actions/pricedifference"
	"github.com/Servflow/servflow/pkg/binance/actions/spotorder"
	"github.com/Servflow/servflow/pkg/binance/actions/tradeinfo"
)

// Definitions describes these actions to a host that offers them.
func Definitions() []actions.Definition {
	return []actions.Definition{
		{
			Type:        "binance/getprice",
			Name:        "Binance Get Price",
			Description: "Retrieves current or historical price data for cryptocurrency symbols from Binance",
			Fields: map[string]actions.FieldInfo{
				"integration": {
					Type:        actions.FieldTypeIntegration,
					Label:       "Binance Account",
					Description: "The Binance integration to call",
					Required:    true,
				},
				"symbol": {
					Type:        actions.FieldTypeString,
					Label:       "Symbol",
					Description: "e.g., BTCUSDT",
					Required:    true,
				},
				"price_type": {
					Type:        actions.FieldTypeString,
					Label:       "Price Type",
					Description: "Select price type",
					Required:    true,
					Default:     "current",
					Values:      []string{"current", "24hr"},
				},
			},
			Output: actions.OutputInfo{
				VariantField: "price_type",
				Variants: map[string]actions.OutputInfo{
					"current": {
						Kind: actions.OutputObject,
						Fields: []actions.OutputField{
							{Path: "symbol", Type: "string", Description: "The symbol that was priced."},
							{Path: "price", Type: "number", Description: "The latest price."},
							{Path: "type", Type: "string", Description: "Always \"current\"."},
						},
					},
					"24hr": {
						Kind: actions.OutputObject,
						Fields: []actions.OutputField{
							{Path: "symbol", Type: "string", Description: "The symbol that was priced."},
							{Path: "priceChange", Type: "string", Description: "Absolute change over the last 24 hours."},
							{Path: "priceChangePercent", Type: "string", Description: "Percentage change over the last 24 hours."},
							{Path: "weightedAvgPrice", Type: "string", Description: "Volume-weighted average price."},
							{Path: "lastPrice", Type: "string", Description: "The most recent trade price."},
							{Path: "volume", Type: "string", Description: "Volume traded over the last 24 hours."},
						},
					},
				},
			},
			New: func(config json.RawMessage) (actions.ActionExecutable, error) {
				var cfg getprice.Config
				if err := json.Unmarshal(config, &cfg); err != nil {
					return nil, fmt.Errorf("error creating getprice action: %v", err)
				}
				return getprice.NewExecutable(cfg)
			},
		},

		{
			Type:        "binance/spotorder",
			Name:        "Binance Spot Order",
			Description: "Places spot trading orders for cryptocurrency pairs on Binance",
			Fields: map[string]actions.FieldInfo{
				"integration": {
					Type:        actions.FieldTypeIntegration,
					Label:       "Binance Account",
					Description: "The Binance integration to call",
					Required:    true,
				},
				"symbol": {
					Type:        actions.FieldTypeString,
					Label:       "Symbol",
					Description: "e.g., BTCUSDT",
					Required:    true,
				},
				"side": {
					Type:        actions.FieldTypeString,
					Label:       "Side",
					Description: "Select order side",
					Required:    true,
					Values:      []string{"BUY", "SELL"},
				},
				"quantity": {
					Type:        actions.FieldTypeString,
					Label:       "Quantity",
					Description: "Enter quantity (base asset)",
					Required:    false,
				},
				"quote_order_qty": {
					Type:        actions.FieldTypeString,
					Label:       "Quote Order Quantity",
					Description: "Enter amount to spend (quote asset, MARKET only)",
					Required:    false,
				},
				"type": {
					Type:        actions.FieldTypeString,
					Label:       "Order Type",
					Description: "Select order type",
					Required:    false,
					Default:     "MARKET",
					Values:      []string{"MARKET", "LIMIT", "STOP_LOSS", "STOP_LOSS_LIMIT", "TAKE_PROFIT", "TAKE_PROFIT_LIMIT"},
				},
				"price": {
					Type:        actions.FieldTypeString,
					Label:       "Price",
					Description: "Enter price (required for LIMIT orders)",
					Required:    false,
				},
				"stop_price": {
					Type:        actions.FieldTypeString,
					Label:       "Stop Price",
					Description: "Enter stop price (required for stop orders)",
					Required:    false,
				},
				"time_in_force": {
					Type:        actions.FieldTypeString,
					Label:       "Time In Force",
					Description: "Select time in force",
					Required:    false,
					Default:     "GTC",
					Values:      []string{"GTC", "IOC", "FOK"},
				},
			},
			Output: actions.OutputInfo{
				Kind: actions.OutputObject,
				Fields: []actions.OutputField{
					{Path: "orderId", Type: "number", Description: "Binance's id for the order."},
					{Path: "symbol", Type: "string", Description: "The symbol traded."},
					{Path: "side", Type: "string", Description: "BUY or SELL."},
					{Path: "type", Type: "string", Description: "The order type that was placed."},
					{Path: "status", Type: "string", Description: "The order's status at the time it was placed."},
				},
			},
			New: func(config json.RawMessage) (actions.ActionExecutable, error) {
				var cfg spotorder.Config
				if err := json.Unmarshal(config, &cfg); err != nil {
					return nil, fmt.Errorf("error creating spotorder action: %v", err)
				}
				return spotorder.NewExecutable(cfg)
			},
		},

		{
			Type:        "binance/pricedifference",
			Name:        "Binance Price Difference",
			Description: "Calculates price differences and percentage changes over specified time periods",
			Fields: map[string]actions.FieldInfo{
				"integration": {
					Type:        actions.FieldTypeIntegration,
					Label:       "Binance Account",
					Description: "The Binance integration to call",
					Required:    true,
				},
				"symbol": {
					Type:        actions.FieldTypeString,
					Label:       "Symbol(s)",
					Description: "e.g., BTCUSDT or BTCUSDT,ETHUSDT for multiple",
					Required:    true,
				},
				"interval": {
					Type:        actions.FieldTypeString,
					Label:       "Interval",
					Description: "Select time interval",
					Required:    false,
					Default:     "1h",
					Values:      []string{"1m", "3m", "5m", "15m", "30m", "1h", "2h", "4h", "6h", "8h", "12h", "1d", "3d", "1w", "1M"},
				},
				"period": {
					Type:        actions.FieldTypeString,
					Label:       "Period",
					Description: "Number of periods (default: 24)",
					Required:    false,
					Default:     "24",
				},
			},
			Output: actions.OutputInfo{
				Kind:        actions.OutputObject,
				Description: "One object per symbol: a single object for one symbol, a list when several are asked for.",
				Fields: []actions.OutputField{
					{Path: "symbol", Type: "string", Description: "The symbol compared."},
					{Path: "currentPrice", Type: "number", Description: "The price at the end of the period."},
					{Path: "previousPrice", Type: "number", Description: "The price at the start of the period."},
					{Path: "difference", Type: "number", Description: "Current price minus previous price."},
					{Path: "differencePercent", Type: "number", Description: "The difference as a percentage of the previous price."},
					{Path: "interval", Type: "string", Description: "The candle interval the comparison used."},
					{Path: "priceHistory", Type: "array", Description: "The candles behind the comparison, each with openTime, open, high, low, close, volume, and closeTime."},
				},
			},
			New: func(config json.RawMessage) (actions.ActionExecutable, error) {
				var cfg pricedifference.Config
				if err := json.Unmarshal(config, &cfg); err != nil {
					return nil, fmt.Errorf("error creating pricedifference action: %v", err)
				}
				return pricedifference.NewExecutable(cfg)
			},
		},

		{
			Type:        "binance/tradeinfo",
			Name:        "Binance Trade Info",
			Description: "Retrieves detailed trading information and statistics for cryptocurrency pairs",
			Fields: map[string]actions.FieldInfo{
				"integration": {
					Type:        actions.FieldTypeIntegration,
					Label:       "Binance Account",
					Description: "The Binance integration to call",
					Required:    true,
				},
				"symbol": {
					Type:        actions.FieldTypeString,
					Label:       "Symbol",
					Description: "e.g., BTCUSDT",
					Required:    true,
				},
			},
			Output: actions.OutputInfo{
				Kind:        actions.OutputDynamic,
				Description: "Open orders under orders, or open positions under positions when reading futures positions. A single order when one is asked for by id.",
			},
			New: func(config json.RawMessage) (actions.ActionExecutable, error) {
				var cfg tradeinfo.Config
				if err := json.Unmarshal(config, &cfg); err != nil {
					return nil, fmt.Errorf("error creating tradeinfo action: %v", err)
				}
				return tradeinfo.NewExecutable(cfg)
			},
		},

		{
			Type:        "binance/accountbalance",
			Name:        "Binance Account Balance",
			Description: "Retrieves current account balance information for all assets",
			Fields: map[string]actions.FieldInfo{
				"integration": {
					Type:        actions.FieldTypeIntegration,
					Label:       "Binance Account",
					Description: "The Binance integration to call",
					Required:    true,
				},
				"symbol": {
					Type:        actions.FieldTypeString,
					Label:       "Symbol",
					Description: "e.g., BTC or BTC,ETH,USDT (leave empty for all)",
					Required:    false,
				},
				"futures": {
					Type:        actions.FieldTypeBoolean,
					Label:       "Futures",
					Description: "Query futures account instead of spot",
					Required:    false,
					Default:     false,
				},
			},
			Output: actions.OutputInfo{
				Kind:        actions.OutputObject,
				Description: "One object per symbol: a single object when one symbol is asked for, a list otherwise.",
				Fields: []actions.OutputField{
					{Path: "symbol", Type: "string", Description: "The asset held."},
					{Path: "balance", Type: "string", Description: "How much of it is held."},
					{Path: "time", Type: "number", Description: "When the balance was read, in milliseconds."},
				},
			},
			New: func(config json.RawMessage) (actions.ActionExecutable, error) {
				var cfg accountbalance.Config
				if err := json.Unmarshal(config, &cfg); err != nil {
					return nil, fmt.Errorf("error creating accountbalance action: %v", err)
				}
				return accountbalance.NewExecutable(cfg)
			},
		},

		{
			Type:        "binance/futuresorder",
			Name:        "Binance Futures Order",
			Description: "Places futures trading orders with leverage and position management on Binance",
			Fields: map[string]actions.FieldInfo{
				"integration": {
					Type:        actions.FieldTypeIntegration,
					Label:       "Binance Account",
					Description: "The Binance integration to call",
					Required:    true,
				},
				"symbol": {
					Type:        actions.FieldTypeString,
					Label:       "Symbol",
					Description: "e.g., BTCUSDT",
					Required:    true,
				},
				"side": {
					Type:        actions.FieldTypeString,
					Label:       "Side",
					Description: "Select order side",
					Required:    true,
					Values:      []string{"BUY", "SELL"},
				},
				"quantity": {
					Type:        actions.FieldTypeString,
					Label:       "Quantity",
					Description: "Enter quantity",
					Required:    true,
				},
				"type": {
					Type:        actions.FieldTypeString,
					Label:       "Order Type",
					Description: "Select order type",
					Required:    false,
					Default:     "MARKET",
					Values:      []string{"MARKET", "LIMIT", "STOP", "STOP_MARKET", "TAKE_PROFIT", "TAKE_PROFIT_MARKET"},
				},
				"price": {
					Type:        actions.FieldTypeString,
					Label:       "Price",
					Description: "Enter price (required for LIMIT orders)",
					Required:    false,
				},
				"time_in_force": {
					Type:        actions.FieldTypeString,
					Label:       "Time In Force",
					Description: "Select time in force",
					Required:    false,
					Default:     "GTC",
					Values:      []string{"GTC", "IOC", "FOK"},
				},
				"position_side": {
					Type:        actions.FieldTypeString,
					Label:       "Position Side",
					Description: "Select position side",
					Required:    false,
					Default:     "BOTH",
					Values:      []string{"BOTH", "LONG", "SHORT"},
				},
				"reduce_only": {
					Type:        actions.FieldTypeString,
					Label:       "Reduce Only",
					Description: "true or false",
					Required:    false,
					Default:     "false",
				},
				"leverage": {
					Type:        actions.FieldTypeString,
					Label:       "Leverage",
					Description: "Enter leverage (optional)",
					Required:    false,
				},
			},
			Output: actions.OutputInfo{
				Kind: actions.OutputObject,
				Fields: []actions.OutputField{
					{Path: "orderId", Type: "number", Description: "Binance's id for the order."},
					{Path: "symbol", Type: "string", Description: "The symbol traded."},
					{Path: "side", Type: "string", Description: "BUY or SELL."},
					{Path: "type", Type: "string", Description: "The order type that was placed."},
					{Path: "executedQty", Type: "string", Description: "How much of the order filled."},
					{Path: "status", Type: "string", Description: "The order's status at the time it was placed."},
					{Path: "positionSide", Type: "string", Description: "LONG, SHORT, or BOTH."},
					{Path: "reduceOnly", Type: "boolean", Description: "Whether the order may only reduce an open position."},
				},
			},
			New: func(config json.RawMessage) (actions.ActionExecutable, error) {
				var cfg futuresorder.Config
				if err := json.Unmarshal(config, &cfg); err != nil {
					return nil, fmt.Errorf("error creating futuresorder action: %v", err)
				}
				return futuresorder.NewExecutable(cfg)
			},
		},
	}
}
