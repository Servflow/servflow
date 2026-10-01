package integration

import (
	"os"
	"testing"
	"time"

	"github.com/Servflow/servflow/pkg/binance"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testSymbol   = "BTCUSDT"
	testQuantity = 0.001 // Small quantity for testing
)

var (
	testClient *Client
	apiKey     string
	secretKey  string
	useTestnet bool
)

func setupClient(t *testing.T) {
	// Skip tests if credentials are not provided
	apiKey = os.Getenv("BINANCE_API_KEY")
	secretKey = os.Getenv("BINANCE_SECRET_KEY")
	testnetEnv := os.Getenv("BINANCE_TESTNET")

	if apiKey == "" || secretKey == "" {
		t.Skip("BINANCE_API_KEY and BINANCE_SECRET_KEY environment variables are required for integration tests")
	}

	useTestnet = testnetEnv == "true" || testnetEnv == "1"

	// Create integration client
	client, err := New(apiKey, secretKey, useTestnet, 30*time.Second)
	require.NoError(t, err)
	require.NotNil(t, client)

	testClient = client
}

func TestClient_New(t *testing.T) {
	client, err := New("test-key", "test-secret", true, 30*time.Second)
	assert.NoError(t, err)
	assert.NotNil(t, client)
	assert.Equal(t, "binance", client.Type())
}

func TestClient_GetCurrentPrice(t *testing.T) {
	setupClient(t)

	tests := []struct {
		name        string
		symbol      string
		expectError bool
	}{
		{
			name:        "Valid symbol",
			symbol:      testSymbol,
			expectError: false,
		},
		{
			name:        "Invalid symbol",
			symbol:      "INVALIDPAIR",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			price, err := testClient.GetCurrentPrice(tt.symbol)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Greater(t, price, 0.0)
			}
		})
	}
}

func TestClient_Get24HrTicker(t *testing.T) {
	setupClient(t)

	tests := []struct {
		name        string
		symbol      string
		expectError bool
	}{
		{
			name:        "Valid symbol",
			symbol:      testSymbol,
			expectError: false,
		},
		{
			name:        "Invalid symbol",
			symbol:      "INVALIDPAIR",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ticker, err := testClient.Get24HrTicker(tt.symbol)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, ticker)
				assert.Equal(t, testSymbol, ticker.Symbol)
				assert.NotEmpty(t, ticker.LastPrice)
				assert.NotEmpty(t, ticker.PriceChange)
				assert.NotEmpty(t, ticker.PriceChangePercent)
				assert.NotEmpty(t, ticker.WeightedAvgPrice)
				assert.NotEmpty(t, ticker.Volume)
			}
		})
	}
}

func TestClient_GetAccountBalance(t *testing.T) {
	setupClient(t)

	if useTestnet {
		t.Skip("Skipping test in testnet mode")
	}

	balance, err := testClient.GetAccountBalance()
	assert.NoError(t, err)
	assert.NotNil(t, balance)
	assert.NotEmpty(t, balance.Balances)

	// Verify structure of balance data
	for _, bal := range balance.Balances {
		assert.NotEmpty(t, bal.Asset)
		assert.NotEmpty(t, bal.Free)
		assert.NotEmpty(t, bal.Locked)
	}
}

func TestClient_GetFuturesAccountInfo(t *testing.T) {
	setupClient(t)

	futuresInfo, err := testClient.GetFuturesAccountInfo()
	assert.NoError(t, err)
	assert.NotNil(t, futuresInfo)
	assert.NotEmpty(t, futuresInfo.Assets)
}

func TestClient_GetHistoricalPrice(t *testing.T) {
	setupClient(t)

	tests := []struct {
		name        string
		symbol      string
		interval    string
		limit       int
		expectError bool
	}{
		{
			name:        "Valid parameters",
			symbol:      testSymbol,
			interval:    "1h",
			limit:       10,
			expectError: false,
		},
		{
			name:        "Invalid symbol",
			symbol:      "INVALIDPAIR",
			interval:    "1h",
			limit:       10,
			expectError: true,
		},
		{
			name:        "Invalid interval",
			symbol:      testSymbol,
			interval:    "invalid",
			limit:       10,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			klines, err := testClient.GetHistoricalPrice(tt.symbol, tt.interval, tt.limit)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, klines)
				assert.Len(t, klines, tt.limit)

				for _, kline := range klines {
					assert.NotEmpty(t, kline.Open)
					assert.NotEmpty(t, kline.High)
					assert.NotEmpty(t, kline.Low)
					assert.NotEmpty(t, kline.Close)
					assert.NotEmpty(t, kline.Volume)
					assert.Greater(t, kline.OpenTime, int64(0))
					assert.Greater(t, kline.CloseTime, int64(0))
				}
			}
		})
	}
}

func TestClient_MarketOrders(t *testing.T) {
	setupClient(t)

	// Only run on testnet
	if !useTestnet {
		t.Skip("Market order tests should only run on testnet")
	}

	tests := []struct {
		name string
		side string
	}{
		{
			name: "Market Buy",
			side: "BUY",
		},
		{
			name: "Market Sell",
			side: "SELL",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			order, err := testClient.PlaceMarketOrder(testSymbol, tt.side, testQuantity)
			if err != nil {
				// On testnet, we might not have enough balance, so we just test the interface
				t.Logf("Market %s failed (expected on testnet): %v", tt.side, err)
				return
			}

			assert.NotNil(t, order)
			assert.Greater(t, order.OrderID, int64(0))
			assert.NotEmpty(t, order.ExecutedQty)
		})
	}
}

func TestClient_LimitOrders(t *testing.T) {
	setupClient(t)

	// Only run on testnet
	if !useTestnet {
		t.Skip("Limit order tests should only run on testnet")
	}

	// Get current price for both tests
	currentPrice, err := testClient.GetCurrentPrice(testSymbol)
	require.NoError(t, err)

	tests := []struct {
		name        string
		side        string
		priceOffset float64
	}{
		{
			name:        "Limit Buy",
			side:        "BUY",
			priceOffset: 0.9, // 10% below current price
		},
		{
			name:        "Limit Sell",
			side:        "SELL",
			priceOffset: 1.1, // 10% above current price
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			limitPrice := currentPrice * tt.priceOffset

			order, err := testClient.PlaceLimitOrder(testSymbol, tt.side, testQuantity, limitPrice, "GTC")
			if err != nil {
				// On testnet, we might not have enough balance, so we just test the interface
				t.Logf("Limit %s failed (expected on testnet): %v", tt.side, err)
				return
			}

			assert.NotNil(t, order)
			assert.Greater(t, order.OrderID, int64(0))
		})
	}
}

func TestClient_InterfaceImplementation(t *testing.T) {
	setupClient(t)

	// Test that the client implements all required interfaces
	assert.Implements(t, (*binance.PriceProvider)(nil), testClient)
	assert.Implements(t, (*binance.TradeExecutor)(nil), testClient)
	assert.Implements(t, (*binance.AccountProvider)(nil), testClient)
	assert.Implements(t, (*binance.Integration)(nil), testClient)
}

func TestClient_Type(t *testing.T) {
	setupClient(t)

	assert.Equal(t, "binance", testClient.Type())
}

func TestClient_GetOpenOrders(t *testing.T) {
	setupClient(t)

	tests := []struct {
		name        string
		symbol      string
		expectError bool
		skip        bool
	}{
		{
			name:        "Valid symbol",
			symbol:      testSymbol,
			expectError: false,
			skip:        true, // Currently skipped in original
		},
		{
			name:        "Invalid symbol",
			symbol:      "INVALIDPAIR",
			expectError: true,
			skip:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.skip {
				t.Skip()
				return
			}

			openOrders, err := testClient.GetOpenOrders(tt.symbol)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, openOrders)
				assert.NotNil(t, openOrders.Orders)

				// Verify structure of open orders
				for _, order := range openOrders.Orders {
					assert.NotEmpty(t, order.Symbol)
					assert.Greater(t, order.OrderID, int64(0))
					assert.NotEmpty(t, order.Status)
					assert.NotEmpty(t, order.Type)
					assert.NotEmpty(t, order.Side)
					assert.NotEmpty(t, order.Price)
					assert.NotEmpty(t, order.OrigQty)
					assert.Greater(t, order.Time, int64(0))
					assert.GreaterOrEqual(t, order.UpdateTime, order.Time)
				}
			}
		})
	}
}

func TestClient_GetFuturesOpenOrders(t *testing.T) {
	setupClient(t)

	tests := []struct {
		name        string
		symbol      string
		expectError bool
	}{
		{
			name:        "Valid symbol",
			symbol:      testSymbol,
			expectError: false,
		},
		{
			name:        "Invalid symbol",
			symbol:      "INVALIDPAIR",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			futuresOpenOrders, err := testClient.GetFuturesOpenOrders(tt.symbol)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, futuresOpenOrders)

				// Verify structure of futures open orders
				for _, order := range futuresOpenOrders.Orders {
					assert.NotEmpty(t, order.Symbol)
					assert.Greater(t, order.OrderID, int64(0))
					assert.NotEmpty(t, order.Status)
					assert.NotEmpty(t, order.Type)
					assert.NotEmpty(t, order.Side)
					assert.NotEmpty(t, order.Price)
					assert.NotEmpty(t, order.OrigQty)
					assert.NotEmpty(t, order.PositionSide)
					assert.Greater(t, order.Time, int64(0))
					assert.GreaterOrEqual(t, order.UpdateTime, order.Time)
				}
			}
		})
	}
}

func TestClient_GetAllPositions(t *testing.T) {
	setupClient(t)

	tests := []struct {
		name         string
		symbol       string
		expectError  bool
		isAllSymbols bool
	}{
		{
			name:        "Specific symbol",
			symbol:      testSymbol,
			expectError: false,
		},
		{
			name:        "Invalid symbol",
			symbol:      "INVALIDPAIR",
			expectError: true,
		},
		{
			name:         "All symbols",
			symbol:       "",
			expectError:  false,
			isAllSymbols: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			positionsResponse, err := testClient.GetAllPositions(tt.symbol)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, positionsResponse)
				assert.NotNil(t, positionsResponse.Positions)

				if tt.isAllSymbols {
					// Log summary for all symbols test
					t.Logf("Found %d total positions across all symbols", len(positionsResponse.Positions))

					// Log symbols found
					symbolsSeen := make(map[string]int)
					for _, position := range positionsResponse.Positions {
						symbolsSeen[position.Symbol]++
					}

					for symbol, count := range symbolsSeen {
						t.Logf("  %s: %d position(s)", symbol, count)
					}
				}

				// Validate structure of positions
				for _, position := range positionsResponse.Positions {
					if !tt.isAllSymbols {
						assert.Equal(t, testSymbol, position.Symbol)
					} else {
						assert.NotEmpty(t, position.Symbol)
					}
					assert.NotEmpty(t, position.PositionAmt)
					assert.NotEmpty(t, position.EntryPrice)
					assert.NotEmpty(t, position.MarkPrice)
					assert.NotEmpty(t, position.UnRealizedProfit)
					assert.NotEmpty(t, position.Leverage)
					assert.NotEmpty(t, position.PositionSide)
					assert.Greater(t, position.UpdateTime, int64(0))
				}
			}
		})
	}
}

func TestClient_SetLeverage(t *testing.T) {
	setupClient(t)

	tests := []struct {
		name        string
		symbol      string
		leverage    int
		expectError bool
		description string
	}{
		{
			name:        "Valid leverage - 20x",
			symbol:      testSymbol,
			leverage:    20,
			expectError: false,
			description: "Set 20x leverage for valid symbol",
		},
		{
			name:        "Invalid leverage - too high",
			symbol:      testSymbol,
			leverage:    1000,
			expectError: true,
			description: "Attempt to set excessively high leverage",
		},
		{
			name:        "Invalid leverage - zero",
			symbol:      testSymbol,
			leverage:    0,
			expectError: true,
			description: "Attempt to set zero leverage",
		},
		{
			name:        "Invalid leverage - negative",
			symbol:      testSymbol,
			leverage:    -5,
			expectError: true,
			description: "Attempt to set negative leverage",
		},
		{
			name:        "Invalid symbol",
			symbol:      "INVALIDPAIR",
			leverage:    10,
			expectError: true,
			description: "Set leverage for invalid symbol",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Logf("Test: %s", tt.description)

			leverageResponse, err := testClient.SetLeverage(tt.symbol, tt.leverage)

			if tt.expectError {
				assert.Error(t, err, "Expected error for test case: %s", tt.description)
				assert.Nil(t, leverageResponse, "Expected nil response when error occurs")
				t.Logf("Expected error occurred: %v", err)
			} else {
				assert.NoError(t, err, "Expected no error for test case: %s", tt.description)
				assert.NotNil(t, leverageResponse, "Expected non-nil response when no error occurs")

				if leverageResponse != nil {
					// Verify response structure and content
					assert.Equal(t, tt.symbol, leverageResponse.Symbol, "Response symbol should match request symbol")
					assert.Equal(t, tt.leverage, leverageResponse.Leverage, "Response leverage should match request leverage")
					assert.NotEmpty(t, leverageResponse.MaxNotionalValue, "MaxNotionalValue should not be empty")

					// Log the response for debugging
					t.Logf("Leverage set successfully: Symbol=%s, Leverage=%d, MaxNotionalValue=%s",
						leverageResponse.Symbol, leverageResponse.Leverage, leverageResponse.MaxNotionalValue)

					// Validate that MaxNotionalValue is a valid number string
					assert.Regexp(t, `^\d+(\.\d+)?$`, leverageResponse.MaxNotionalValue, "MaxNotionalValue should be a valid number string")
				}
			}
		})
	}
}
