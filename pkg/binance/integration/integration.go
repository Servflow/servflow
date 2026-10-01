package integration

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	binance2 "github.com/Servflow/servflow/pkg/binance"
	"github.com/Servflow/servflow/pkg/engine/integration"
	"github.com/adshao/go-binance/v2"
	"github.com/adshao/go-binance/v2/futures"
)

type Config struct {
	APIKey    string `json:"api_key"`
	SecretKey string `json:"secret_key"`
	Testnet   bool   `json:"testnet,omitempty"`
	Timeout   int    `json:"timeout,omitempty"`
}

type Client struct {
	integration.BaseIntegration
	binanceClient *binance.Client
	futuresClient *futures.Client
	config        Config
}

func (c *Client) Type() string {
	return "binance"
}

func (c *Client) GetCurrentPrice(symbol string) (float64, error) {
	prices, err := c.binanceClient.NewListPricesService().Symbol(symbol).Do(context.Background())
	if err != nil {
		return 0, err
	}
	if len(prices) == 0 {
		return 0, errors.New("no price data found")
	}
	return strconv.ParseFloat(prices[0].Price, 64)
}

func (c *Client) Get24HrTicker(symbol string) (*binance2.TickerStats, error) {
	ticker, err := c.binanceClient.NewListPriceChangeStatsService().Symbol(symbol).Do(context.Background())
	if err != nil {
		return nil, err
	}
	if len(ticker) == 0 {
		return nil, errors.New("no ticker data found")
	}

	return &binance2.TickerStats{
		Symbol:             ticker[0].Symbol,
		PriceChange:        ticker[0].PriceChange,
		PriceChangePercent: ticker[0].PriceChangePercent,
		WeightedAvgPrice:   ticker[0].WeightedAvgPrice,
		LastPrice:          ticker[0].LastPrice,
		Volume:             ticker[0].Volume,
	}, nil
}

func (c *Client) PlaceMarketOrder(symbol, side string, quantity float64) (*binance2.OrderResponse, error) {
	var order *binance.CreateOrderResponse
	var err error

	service := c.binanceClient.NewCreateOrderService().
		Symbol(symbol).
		Side(binance.SideType(side)).
		Type(binance.OrderTypeMarket).
		Quantity(fmt.Sprintf("%.8f", quantity))

	order, err = service.Do(context.Background())
	if err != nil {
		return nil, err
	}

	return &binance2.OrderResponse{
		Symbol:        order.Symbol,
		OrderID:       order.OrderID,
		ClientOrderID: order.ClientOrderID,
		Price:         order.Price,
		OrigQty:       order.OrigQuantity,
		ExecutedQty:   order.ExecutedQuantity,
		Status:        string(order.Status),
		Side:          string(order.Side),
		Type:          string(order.Type),
	}, nil
}

func (c *Client) PlaceLimitOrder(symbol, side string, quantity, price float64, timeInForce string) (*binance2.OrderResponse, error) {
	service := c.binanceClient.NewCreateOrderService().
		Symbol(symbol).
		Side(binance.SideType(side)).
		Type(binance.OrderTypeLimit).
		Quantity(fmt.Sprintf("%.8f", quantity)).
		Price(fmt.Sprintf("%.8f", price)).
		TimeInForce(binance.TimeInForceType(timeInForce))

	order, err := service.Do(context.Background())
	if err != nil {
		return nil, err
	}

	return &binance2.OrderResponse{
		Symbol:        order.Symbol,
		OrderID:       order.OrderID,
		ClientOrderID: order.ClientOrderID,
		Price:         order.Price,
		OrigQty:       order.OrigQuantity,
		ExecutedQty:   order.ExecutedQuantity,
		Status:        string(order.Status),
		Side:          string(order.Side),
		Type:          string(order.Type),
	}, nil
}

func (c *Client) PlaceSpotOrder(symbol, side string, quantity, quoteOrderQty *float64, orderType string, price, stopPrice *float64, timeInForce string) (*binance2.OrderResponse, error) {
	service := c.binanceClient.NewCreateOrderService().
		Symbol(symbol).
		Side(binance.SideType(side)).
		Type(binance.OrderType(orderType))

	if quantity != nil {
		service.Quantity(fmt.Sprintf("%.8f", *quantity))
	}

	if quoteOrderQty != nil {
		service.QuoteOrderQty(fmt.Sprintf("%.8f", *quoteOrderQty))
	}

	if price != nil {
		service.Price(fmt.Sprintf("%.8f", *price))
	}

	if stopPrice != nil {
		service.StopPrice(fmt.Sprintf("%.8f", *stopPrice))
	}

	if timeInForce != "" {
		service.TimeInForce(binance.TimeInForceType(timeInForce))
	}

	order, err := service.Do(context.Background())
	if err != nil {
		return nil, err
	}

	return &binance2.OrderResponse{
		Symbol:        order.Symbol,
		OrderID:       order.OrderID,
		ClientOrderID: order.ClientOrderID,
		Price:         order.Price,
		OrigQty:       order.OrigQuantity,
		ExecutedQty:   order.ExecutedQuantity,
		Status:        string(order.Status),
		Side:          string(order.Side),
		Type:          string(order.Type),
	}, nil
}

func (c *Client) GetOpenOrders(symbol string) (*binance2.OpenOrdersResponse, error) {
	orders, err := c.binanceClient.NewListOpenOrdersService().Symbol(symbol).Do(context.Background())
	if err != nil {
		return nil, err
	}

	var orderInfos []*binance2.OrderInfo
	for _, order := range orders {
		orderInfos = append(orderInfos, &binance2.OrderInfo{
			Symbol:        order.Symbol,
			OrderID:       order.OrderID,
			ClientOrderID: order.ClientOrderID,
			Price:         order.Price,
			OrigQty:       order.OrigQuantity,
			ExecutedQty:   order.ExecutedQuantity,
			Status:        string(order.Status),
			TimeInForce:   string(order.TimeInForce),
			Type:          string(order.Type),
			Side:          string(order.Side),
			Time:          order.Time,
			UpdateTime:    order.UpdateTime,
		})
	}

	return &binance2.OpenOrdersResponse{
		Orders: orderInfos,
	}, nil
}

func (c *Client) GetHistoricalPrice(symbol string, interval string, limit int) ([]*binance2.KlineData, error) {
	klines, err := c.binanceClient.NewKlinesService().Symbol(symbol).Interval(interval).Limit(limit).Do(context.Background())
	if err != nil {
		return nil, err
	}

	var result []*binance2.KlineData
	for _, kline := range klines {
		result = append(result, &binance2.KlineData{
			OpenTime:  kline.OpenTime,
			Open:      kline.Open,
			High:      kline.High,
			Low:       kline.Low,
			Close:     kline.Close,
			Volume:    kline.Volume,
			CloseTime: kline.CloseTime,
		})
	}

	return result, nil
}

func (c *Client) GetOrderInfo(symbol string, orderID int64) (*binance2.OrderInfo, error) {
	order, err := c.binanceClient.NewGetOrderService().Symbol(symbol).OrderID(orderID).Do(context.Background())
	if err != nil {
		return nil, err
	}

	return &binance2.OrderInfo{
		Symbol:        order.Symbol,
		OrderID:       order.OrderID,
		ClientOrderID: order.ClientOrderID,
		Price:         order.Price,
		OrigQty:       order.OrigQuantity,
		ExecutedQty:   order.ExecutedQuantity,
		Status:        string(order.Status),
		TimeInForce:   string(order.TimeInForce),
		Type:          string(order.Type),
		Side:          string(order.Side),
		Time:          order.Time,
		UpdateTime:    order.UpdateTime,
	}, nil
}

func (c *Client) GetAccountBalance() (*binance2.AccountBalance, error) {
	account, err := c.binanceClient.NewGetAccountService().Do(context.Background())
	if err != nil {
		return nil, err
	}

	var balances []*binance2.Balance
	for _, balance := range account.Balances {
		balances = append(balances, &binance2.Balance{
			Asset:  balance.Asset,
			Free:   balance.Free,
			Locked: balance.Locked,
		})
	}

	return &binance2.AccountBalance{
		MakerCommission:  int(account.MakerCommission),
		TakerCommission:  int(account.TakerCommission),
		BuyerCommission:  int(account.BuyerCommission),
		SellerCommission: int(account.SellerCommission),
		CanTrade:         account.CanTrade,
		CanWithdraw:      account.CanWithdraw,
		CanDeposit:       account.CanDeposit,
		UpdateTime:       int64(account.UpdateTime),
		AccountType:      account.AccountType,
		Balances:         balances,
	}, nil
}

// PlaceFuturesOrder places a futures order with advanced parameters including reduceOnly and positionSide
func (c *Client) PlaceFuturesOrder(symbol, side string, quantity float64, orderType string, price *float64, timeInForce, positionSide string, reduceOnly bool) (*binance2.FuturesOrderResponse, error) {
	service := c.futuresClient.NewCreateOrderService().
		Symbol(symbol).
		Side(futures.SideType(side)).
		Type(futures.OrderType(orderType)).
		Quantity(fmt.Sprintf("%.8f", quantity))

	if orderType == "LIMIT" && price != nil {
		service.Price(fmt.Sprintf("%.8f", *price))
		if timeInForce != "" {
			service.TimeInForce(futures.TimeInForceType(timeInForce))
		}
	}

	// Set position side if specified
	if positionSide != "" {
		service.PositionSide(futures.PositionSideType(positionSide))
	}

	// Set reduce only flag
	if reduceOnly {
		service.ReduceOnly(reduceOnly)
	}

	order, err := service.Do(context.Background())
	if err != nil {
		return nil, err
	}

	return &binance2.FuturesOrderResponse{
		Symbol:        order.Symbol,
		OrderID:       order.OrderID,
		ClientOrderID: order.ClientOrderID,
		Price:         order.Price,
		OrigQty:       order.OrigQuantity,
		ExecutedQty:   order.ExecutedQuantity,
		Status:        string(order.Status),
		Side:          string(order.Side),
		Type:          string(order.Type),
		PositionSide:  string(order.PositionSide),
		ReduceOnly:    order.ReduceOnly,
	}, nil
}

// GetFuturesAccountInfo retrieves futures account information
func (c *Client) GetFuturesAccountInfo() (*binance2.FuturesAccountInfo, error) {
	account, err := c.futuresClient.NewGetAccountService().Do(context.Background())
	if err != nil {
		return nil, err
	}

	var assets []*binance2.FuturesAccountAsset
	for _, asset := range account.Assets {
		assets = append(assets, &binance2.FuturesAccountAsset{
			Asset:                  asset.Asset,
			WalletBalance:          asset.WalletBalance,
			UnrealizedProfit:       asset.UnrealizedProfit,
			MarginBalance:          asset.MarginBalance,
			MaintMargin:            asset.MaintMargin,
			InitialMargin:          asset.InitialMargin,
			PositionInitialMargin:  asset.PositionInitialMargin,
			OpenOrderInitialMargin: asset.OpenOrderInitialMargin,
			CrossWalletBalance:     asset.CrossWalletBalance,
			CrossUnPnl:             asset.CrossUnPnl,
			AvailableBalance:       asset.AvailableBalance,
			MaxWithdrawAmount:      asset.MaxWithdrawAmount,
			MarginAvailable:        asset.MarginAvailable,
			UpdateTime:             asset.UpdateTime,
		})
	}

	var positions []*binance2.FuturesPosition
	for _, position := range account.Positions {
		positions = append(positions, &binance2.FuturesPosition{
			Symbol:                 position.Symbol,
			InitialMargin:          position.InitialMargin,
			MaintMargin:            position.MaintMargin,
			UnrealizedProfit:       position.UnrealizedProfit,
			PositionInitialMargin:  position.PositionInitialMargin,
			OpenOrderInitialMargin: position.OpenOrderInitialMargin,
			Leverage:               position.Leverage,
			Isolated:               position.Isolated,
			EntryPrice:             position.EntryPrice,
			MaxNotional:            position.MaxNotional,
			BidNotional:            position.BidNotional,
			AskNotional:            position.AskNotional,
			PositionSide:           string(position.PositionSide),
			PositionAmt:            position.PositionAmt,
			UpdateTime:             0, // UpdateTime not available in PositionRisk
		})
	}

	return &binance2.FuturesAccountInfo{
		FeeTier:                     int(account.FeeTier),
		CanTrade:                    account.CanTrade,
		CanDeposit:                  account.CanDeposit,
		CanWithdraw:                 account.CanWithdraw,
		UpdateTime:                  account.UpdateTime,
		TotalInitialMargin:          account.TotalInitialMargin,
		TotalMaintMargin:            account.TotalMaintMargin,
		TotalWalletBalance:          account.TotalWalletBalance,
		TotalUnrealizedProfit:       account.TotalUnrealizedProfit,
		TotalMarginBalance:          account.TotalMarginBalance,
		TotalPositionInitialMargin:  account.TotalPositionInitialMargin,
		TotalOpenOrderInitialMargin: account.TotalOpenOrderInitialMargin,
		TotalCrossWalletBalance:     account.TotalCrossWalletBalance,
		TotalCrossUnPnl:             account.TotalCrossUnPnl,
		AvailableBalance:            account.AvailableBalance,
		MaxWithdrawAmount:           account.MaxWithdrawAmount,
		Assets:                      assets,
		Positions:                   positions,
	}, nil
}

// SetLeverage sets leverage for a futures symbol
func (c *Client) SetLeverage(symbol string, leverage int) (*binance2.LeverageResponse, error) {
	result, err := c.futuresClient.NewChangeLeverageService().
		Symbol(symbol).
		Leverage(leverage).
		Do(context.Background())
	if err != nil {
		return nil, err
	}

	return &binance2.LeverageResponse{
		Leverage:         result.Leverage,
		MaxNotionalValue: result.MaxNotionalValue,
		Symbol:           result.Symbol,
	}, nil
}

// GetAllPositions retrieves all position information for a symbol, or all positions if symbol is empty
func (c *Client) GetAllPositions(symbol string) (*binance2.PositionsResponse, error) {
	service := c.futuresClient.NewGetPositionRiskService()
	if symbol != "" {
		service = service.Symbol(symbol)
	}

	positions, err := service.Do(context.Background())
	if err != nil {
		return nil, err
	}

	var positionInfos []*binance2.PositionInfo
	for _, position := range positions {
		positionInfos = append(positionInfos, &binance2.PositionInfo{
			Symbol:           position.Symbol,
			PositionAmt:      position.PositionAmt,
			EntryPrice:       position.EntryPrice,
			MarkPrice:        position.MarkPrice,
			UnRealizedProfit: position.UnRealizedProfit,
			LiquidationPrice: position.LiquidationPrice,
			Leverage:         position.Leverage,
			MaxNotionalValue: position.MaxNotionalValue,
			MarginType:       position.MarginType,
			IsolatedMargin:   position.IsolatedMargin,
			IsAutoAddMargin:  position.IsAutoAddMargin,
			PositionSide:     position.PositionSide,
			Notional:         position.Notional,
			IsolatedWallet:   position.IsolatedWallet,
			UpdateTime:       time.Now().Unix(), // Use current time as UpdateTime is not available in PositionRisk
		})
	}

	return &binance2.PositionsResponse{
		Positions: positionInfos,
	}, nil
}

// GetFuturesOpenOrders retrieves all open futures orders for a symbol, or all open orders if symbol is empty
func (c *Client) GetFuturesOpenOrders(symbol string) (*binance2.FuturesOpenOrdersResponse, error) {
	service := c.futuresClient.NewListOpenOrdersService()
	if symbol != "" {
		service = service.Symbol(symbol)
	}
	orders, err := service.Do(context.Background())
	if err != nil {
		return nil, err
	}

	var orderInfos []*binance2.FuturesOrderInfo
	for _, order := range orders {
		orderInfos = append(orderInfos, &binance2.FuturesOrderInfo{
			Symbol:        order.Symbol,
			OrderID:       order.OrderID,
			ClientOrderID: order.ClientOrderID,
			Price:         order.Price,
			OrigQty:       order.OrigQuantity,
			ExecutedQty:   order.ExecutedQuantity,
			Status:        string(order.Status),
			TimeInForce:   string(order.TimeInForce),
			Type:          string(order.Type),
			Side:          string(order.Side),
			PositionSide:  string(order.PositionSide),
			ReduceOnly:    order.ReduceOnly,
			Time:          order.Time,
			UpdateTime:    order.UpdateTime,
		})
	}

	return &binance2.FuturesOpenOrdersResponse{
		Orders: orderInfos,
	}, nil
}

func New(apiKey, secretKey string, testnet bool, timeout time.Duration) (*Client, error) {
	if apiKey == "" {
		return nil, errors.New("API key is required")
	}
	if secretKey == "" {
		return nil, errors.New("secret key is required")
	}

	binance.UseTestnet = testnet
	futures.UseTestnet = testnet

	client := binance.NewClient(apiKey, secretKey)
	futuresClient := binance.NewFuturesClient(apiKey, secretKey)

	return &Client{
		binanceClient: client,
		futuresClient: futuresClient,
		config: Config{
			APIKey:    apiKey,
			SecretKey: secretKey,
			Testnet:   testnet,
			Timeout:   int(timeout.Seconds()),
		},
	}, nil
}

// Definition describes the binance integration to a host that offers it.
func Definition() integration.Definition {
	fields := map[string]integration.FieldInfo{
		"api_key": {
			Type:        integration.FieldTypePassword,
			Label:       "API Key",
			Description: "Your Binance API key",
			Required:    true,
		},
		"secret_key": {
			Type:        integration.FieldTypePassword,
			Label:       "Secret Key",
			Description: "Your Binance secret key",
			Required:    true,
		},
		"testnet": {
			Type:        integration.FieldTypeBoolean,
			Label:       "Testnet",
			Description: "Use testnet",
			Required:    false,
			Default:     false,
		},
		"timeout": {
			Type:        integration.FieldTypeNumber,
			Label:       "Timeout (seconds)",
			Description: "30",
			Required:    false,
			Default:     30,
		},
	}

	return integration.Definition{
		Type:        "binance",
		Name:        "Binance",
		Description: "Binance cryptocurrency exchange integration for trading operations",
		ImageURL:    "https://d2ojax9k5fldtt.cloudfront.net/binance.svg",
		Fields:      fields,
		New: func(m map[string]any) (integration.Integration, error) {
			apiKey, ok := m["api_key"].(string)
			if !ok {
				return nil, errors.New("api_key required in config")
			}

			secretKey, ok := m["secret_key"].(string)
			if !ok {
				return nil, errors.New("secret_key required in config")
			}

			testnet, _ := m["testnet"].(bool)
			timeout := 30 * time.Second
			if t, ok := m["timeout"].(float64); ok {
				timeout = time.Duration(t) * time.Second
			}

			client, err := New(apiKey, secretKey, testnet, timeout)
			if err != nil {
				return nil, err
			}

			return client, nil
		},
	}
}
