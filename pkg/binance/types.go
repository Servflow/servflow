//go:generate mockgen -source types.go -destination binance_mocks.go -package binance

package binance

import "github.com/Servflow/servflow/pkg/engine/integration"

// Interfaces

type PriceProvider interface {
	integration.Integration
	GetCurrentPrice(symbol string) (float64, error)
	Get24HrTicker(symbol string) (*TickerStats, error)
	GetHistoricalPrice(symbol string, interval string, limit int) ([]*KlineData, error)
}

type TradeExecutor interface {
	integration.Integration
	PlaceMarketOrder(symbol, side string, quantity float64) (*OrderResponse, error)
	PlaceLimitOrder(symbol, side string, quantity, price float64, timeInForce string) (*OrderResponse, error)
	PlaceSpotOrder(symbol, side string, quantity, quoteOrderQty *float64, orderType string, price, stopPrice *float64, timeInForce string) (*OrderResponse, error)
	GetOrderInfo(symbol string, orderID int64) (*OrderInfo, error)
	GetOpenOrders(symbol string) (*OpenOrdersResponse, error)
}

type AccountProvider interface {
	integration.Integration
	GetAccountBalance() (*AccountBalance, error)
}

type FuturesTrader interface {
	integration.Integration
	PlaceFuturesOrder(symbol, side string, quantity float64, orderType string, price *float64, timeInForce, positionSide string, reduceOnly bool) (*FuturesOrderResponse, error)
	GetFuturesAccountInfo() (*FuturesAccountInfo, error)
	GetFuturesOpenOrders(symbol string) (*FuturesOpenOrdersResponse, error)
	SetLeverage(symbol string, leverage int) (*LeverageResponse, error)
	GetAllPositions(symbol string) (*PositionsResponse, error)
}

type Integration interface {
	PriceProvider
	TradeExecutor
	AccountProvider
	FuturesTrader
}

// Types

type MakeTradeConfig struct {
	Integration string `json:"integration"`
	Symbol      string `json:"symbol"`
	Side        string `json:"side"`
	Type        string `json:"type"`
	Quantity    string `json:"quantity"`
	Price       string `json:"price,omitempty"`
	TimeInForce string `json:"time_in_force,omitempty"`
}

type TickerStats struct {
	Symbol             string `json:"symbol"`
	PriceChange        string `json:"priceChange"`
	PriceChangePercent string `json:"priceChangePercent"`
	WeightedAvgPrice   string `json:"weightedAvgPrice"`
	LastPrice          string `json:"lastPrice"`
	Volume             string `json:"volume"`
}

type OrderResponse struct {
	Symbol        string `json:"symbol"`
	OrderID       int64  `json:"orderId"`
	ClientOrderID string `json:"clientOrderId"`
	Price         string `json:"price"`
	OrigQty       string `json:"origQty"`
	ExecutedQty   string `json:"executedQty"`
	Status        string `json:"status"`
	Side          string `json:"side"`
	Type          string `json:"type"`
}

type KlineData struct {
	OpenTime  int64  `json:"openTime"`
	Open      string `json:"open"`
	High      string `json:"high"`
	Low       string `json:"low"`
	Close     string `json:"close"`
	Volume    string `json:"volume"`
	CloseTime int64  `json:"closeTime"`
}

type OrderInfo struct {
	Symbol        string `json:"symbol"`
	OrderID       int64  `json:"orderId"`
	ClientOrderID string `json:"clientOrderId"`
	Price         string `json:"price"`
	OrigQty       string `json:"origQty"`
	ExecutedQty   string `json:"executedQty"`
	Status        string `json:"status"`
	TimeInForce   string `json:"timeInForce"`
	Type          string `json:"type"`
	Side          string `json:"side"`
	Time          int64  `json:"time"`
	UpdateTime    int64  `json:"updateTime"`
}

type Balance struct {
	Asset  string `json:"asset"`
	Free   string `json:"free"`
	Locked string `json:"locked"`
}

type AccountBalance struct {
	MakerCommission  int        `json:"makerCommission"`
	TakerCommission  int        `json:"takerCommission"`
	BuyerCommission  int        `json:"buyerCommission"`
	SellerCommission int        `json:"sellerCommission"`
	CanTrade         bool       `json:"canTrade"`
	CanWithdraw      bool       `json:"canWithdraw"`
	CanDeposit       bool       `json:"canDeposit"`
	UpdateTime       int64      `json:"updateTime"`
	AccountType      string     `json:"accountType"`
	Balances         []*Balance `json:"balances"`
}

type MarginOrderResponse struct {
	Symbol        string `json:"symbol"`
	OrderID       int64  `json:"orderId"`
	ClientOrderID string `json:"clientOrderId"`
	Price         string `json:"price"`
	OrigQty       string `json:"origQty"`
	ExecutedQty   string `json:"executedQty"`
	Status        string `json:"status"`
	Side          string `json:"side"`
	Type          string `json:"type"`
	IsIsolated    bool   `json:"isIsolated"`
}

type MarginAccountInfo struct {
	BorrowEnabled       bool               `json:"borrowEnabled"`
	MarginLevel         string             `json:"marginLevel"`
	TotalAssetOfBtc     string             `json:"totalAssetOfBtc"`
	TotalLiabilityOfBtc string             `json:"totalLiabilityOfBtc"`
	TotalNetAssetOfBtc  string             `json:"totalNetAssetOfBtc"`
	TradeEnabled        bool               `json:"tradeEnabled"`
	TransferEnabled     bool               `json:"transferEnabled"`
	UserAssets          []*MarginUserAsset `json:"userAssets"`
}

type MarginUserAsset struct {
	Asset    string `json:"asset"`
	Borrowed string `json:"borrowed"`
	Free     string `json:"free"`
	Interest string `json:"interest"`
	Locked   string `json:"locked"`
	NetAsset string `json:"netAsset"`
}

type BorrowResponse struct {
	TranID int64 `json:"tranId"`
}

type RepayResponse struct {
	TranID int64 `json:"tranId"`
}

type FuturesOrderResponse struct {
	Symbol        string `json:"symbol"`
	OrderID       int64  `json:"orderId"`
	ClientOrderID string `json:"clientOrderId"`
	Price         string `json:"price"`
	OrigQty       string `json:"origQty"`
	ExecutedQty   string `json:"executedQty"`
	Status        string `json:"status"`
	Side          string `json:"side"`
	Type          string `json:"type"`
	PositionSide  string `json:"positionSide"`
	ReduceOnly    bool   `json:"reduceOnly"`
}

type FuturesAccountInfo struct {
	FeeTier                     int                    `json:"feeTier"`
	CanTrade                    bool                   `json:"canTrade"`
	CanDeposit                  bool                   `json:"canDeposit"`
	CanWithdraw                 bool                   `json:"canWithdraw"`
	UpdateTime                  int64                  `json:"updateTime"`
	TotalInitialMargin          string                 `json:"totalInitialMargin"`
	TotalMaintMargin            string                 `json:"totalMaintMargin"`
	TotalWalletBalance          string                 `json:"totalWalletBalance"`
	TotalUnrealizedProfit       string                 `json:"totalUnrealizedProfit"`
	TotalMarginBalance          string                 `json:"totalMarginBalance"`
	TotalPositionInitialMargin  string                 `json:"totalPositionInitialMargin"`
	TotalOpenOrderInitialMargin string                 `json:"totalOpenOrderInitialMargin"`
	TotalCrossWalletBalance     string                 `json:"totalCrossWalletBalance"`
	TotalCrossUnPnl             string                 `json:"totalCrossUnPnl"`
	AvailableBalance            string                 `json:"availableBalance"`
	MaxWithdrawAmount           string                 `json:"maxWithdrawAmount"`
	Assets                      []*FuturesAccountAsset `json:"assets"`
	Positions                   []*FuturesPosition     `json:"positions"`
}

type FuturesAccountAsset struct {
	Asset                  string `json:"asset"`
	WalletBalance          string `json:"walletBalance"`
	UnrealizedProfit       string `json:"unrealizedProfit"`
	MarginBalance          string `json:"marginBalance"`
	MaintMargin            string `json:"maintMargin"`
	InitialMargin          string `json:"initialMargin"`
	PositionInitialMargin  string `json:"positionInitialMargin"`
	OpenOrderInitialMargin string `json:"openOrderInitialMargin"`
	CrossWalletBalance     string `json:"crossWalletBalance"`
	CrossUnPnl             string `json:"crossUnPnl"`
	AvailableBalance       string `json:"availableBalance"`
	MaxWithdrawAmount      string `json:"maxWithdrawAmount"`
	MarginAvailable        bool   `json:"marginAvailable"`
	UpdateTime             int64  `json:"updateTime"`
}

type FuturesPosition struct {
	Symbol                 string `json:"symbol"`
	InitialMargin          string `json:"initialMargin"`
	MaintMargin            string `json:"maintMargin"`
	UnrealizedProfit       string `json:"unrealizedProfit"`
	PositionInitialMargin  string `json:"positionInitialMargin"`
	OpenOrderInitialMargin string `json:"openOrderInitialMargin"`
	Leverage               string `json:"leverage"`
	Isolated               bool   `json:"isolated"`
	EntryPrice             string `json:"entryPrice"`
	MaxNotional            string `json:"maxNotional"`
	BidNotional            string `json:"bidNotional"`
	AskNotional            string `json:"askNotional"`
	PositionSide           string `json:"positionSide"`
	PositionAmt            string `json:"positionAmt"`
	UpdateTime             int64  `json:"updateTime"`
}

type LeverageResponse struct {
	Leverage         int    `json:"leverage"`
	MaxNotionalValue string `json:"maxNotionalValue"`
	Symbol           string `json:"symbol"`
}

type PositionInfo struct {
	Symbol           string `json:"symbol"`
	PositionAmt      string `json:"positionAmt"`
	EntryPrice       string `json:"entryPrice"`
	MarkPrice        string `json:"markPrice"`
	UnRealizedProfit string `json:"unRealizedProfit"`
	LiquidationPrice string `json:"liquidationPrice"`
	Leverage         string `json:"leverage"`
	MaxNotionalValue string `json:"maxNotionalValue"`
	MarginType       string `json:"marginType"`
	IsolatedMargin   string `json:"isolatedMargin"`
	IsAutoAddMargin  string `json:"isAutoAddMargin"`
	PositionSide     string `json:"positionSide"`
	Notional         string `json:"notional"`
	IsolatedWallet   string `json:"isolatedWallet"`
	UpdateTime       int64  `json:"updateTime"`
}

type FuturesOrderInfo struct {
	Symbol        string `json:"symbol"`
	OrderID       int64  `json:"orderId"`
	ClientOrderID string `json:"clientOrderId"`
	Price         string `json:"price"`
	OrigQty       string `json:"origQty"`
	ExecutedQty   string `json:"executedQty"`
	Status        string `json:"status"`
	TimeInForce   string `json:"timeInForce"`
	Type          string `json:"type"`
	Side          string `json:"side"`
	PositionSide  string `json:"positionSide"`
	ReduceOnly    bool   `json:"reduceOnly"`
	Time          int64  `json:"time"`
	UpdateTime    int64  `json:"updateTime"`
}

type OpenOrdersResponse struct {
	Orders []*OrderInfo `json:"orders"`
}

type FuturesOpenOrdersResponse struct {
	Orders []*FuturesOrderInfo `json:"orders"`
}

type PositionsResponse struct {
	Positions []*PositionInfo `json:"positions"`
}
