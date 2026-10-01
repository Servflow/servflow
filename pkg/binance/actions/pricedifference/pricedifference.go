package pricedifference

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Servflow/servflow/pkg/engine/requestctx"

	"github.com/Servflow/servflow/pkg/binance"
)

// ReadableKlineData represents kline data with human-readable timestamps
type ReadableKlineData struct {
	OpenTime  string `json:"openTime"`
	Open      string `json:"open"`
	High      string `json:"high"`
	Low       string `json:"low"`
	Close     string `json:"close"`
	Volume    string `json:"volume"`
	CloseTime string `json:"closeTime"`
}

// Response represents the response for price difference queries
type Response struct {
	Symbol            string              `json:"symbol"`
	CurrentPrice      float64             `json:"currentPrice"`
	PreviousPrice     float64             `json:"previousPrice"`
	Difference        float64             `json:"difference"`
	DifferencePercent float64             `json:"differencePercent"`
	PriceHistory      []ReadableKlineData `json:"priceHistory"`
	Interval          string              `json:"interval"`
}

type Config struct {
	Integration string `json:"integration"`
	Symbol      string `json:"symbol"` // Can be comma-separated for multiple symbols
	Interval    string `json:"interval"`
	Period      string `json:"period"`
}

type Executable struct {
	config Config
}

func (e *Executable) Type() string {
	return "binance/pricedifference"
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

// formatTimestamp converts Unix timestamp to human-readable format
func formatTimestamp(timestamp int64) string {
	if timestamp == 0 {
		return ""
	}
	return time.Unix(timestamp/1000, 0).Format("2006-01-02 15:04:05 UTC")
}

// convertToReadableKlineData converts kline data to readable format
func convertToReadableKlineData(klines []*binance.KlineData) []ReadableKlineData {
	readablePriceHistory := make([]ReadableKlineData, len(klines))
	for i, kline := range klines {
		readablePriceHistory[i] = ReadableKlineData{
			OpenTime:  formatTimestamp(kline.OpenTime),
			Open:      kline.Open,
			High:      kline.High,
			Low:       kline.Low,
			Close:     kline.Close,
			Volume:    kline.Volume,
			CloseTime: formatTimestamp(kline.CloseTime),
		}
	}
	return readablePriceHistory
}

// calculatePriceDifference calculates the price difference and percentage
func calculatePriceDifference(currentPrice, previousPrice float64) (float64, float64) {
	difference := currentPrice - previousPrice
	differencePercent := 0.0
	if previousPrice != 0 {
		differencePercent = (difference / previousPrice) * 100
	}
	return difference, differencePercent
}

// calculatePriceDifferenceForSymbol calculates price difference for a single symbol
func (e *Executable) calculatePriceDifferenceForSymbol(ctx context.Context, binanceIntegration binance.Integration, symbol, interval string, periods int) (*Response, error) {
	// Get current price
	currentPrice, err := binanceIntegration.GetCurrentPrice(symbol)
	if err != nil {
		return nil, fmt.Errorf("error getting current price for %s: %v", symbol, err)
	}

	// Get historical price data
	klines, err := binanceIntegration.GetHistoricalPrice(symbol, interval, periods)
	if err != nil {
		return nil, fmt.Errorf("error getting historical price data for %s: %v", symbol, err)
	}

	if len(klines) == 0 {
		return nil, fmt.Errorf("no historical price data found for %s", symbol)
	}

	// Get the previous price (oldest data point)
	previousPriceStr := klines[0].Close
	previousPrice, err := strconv.ParseFloat(previousPriceStr, 64)
	if err != nil {
		return nil, fmt.Errorf("error parsing previous price for %s: %v", symbol, err)
	}

	// Calculate difference
	difference, differencePercent := calculatePriceDifference(currentPrice, previousPrice)

	// Convert kline data to readable format
	readablePriceHistory := convertToReadableKlineData(klines)

	return &Response{
		Symbol:            symbol,
		CurrentPrice:      currentPrice,
		PreviousPrice:     previousPrice,
		Difference:        difference,
		DifferencePercent: differencePercent,
		PriceHistory:      readablePriceHistory,
		Interval:          interval,
	}, nil
}

// parseConfig parses the modified config or returns the default config
func (e *Executable) parseConfig(modifiedConfig string) (Config, error) {
	var config Config
	if modifiedConfig != "" {
		if err := json.Unmarshal([]byte(modifiedConfig), &config); err != nil {
			return config, fmt.Errorf("error parsing config: %v", err)
		}
	} else {
		config = e.config
	}
	return config, nil
}

// validateAndNormalizeConfig validates and normalizes the configuration
func validateAndNormalizeConfig(config *Config) (int, error) {
	// Validate required fields
	if config.Symbol == "" {
		return 0, errors.New("symbol is required")
	}
	if config.Interval == "" {
		config.Interval = "1h" // Default to 1 hour
	}

	// Convert periods string to int
	periods := 24 // Default to 24 periods (24 hours for 1h interval)
	if config.Period != "" {
		parsedPeriods, err := strconv.Atoi(config.Period)
		if err != nil {
			return 0, fmt.Errorf("invalid periods format: %v", err)
		}
		if parsedPeriods <= 0 {
			return 0, errors.New("periods must be greater than 0")
		}
		periods = parsedPeriods
	}

	return periods, nil
}

// validateInterval validates the interval format
func validateInterval(interval string) error {
	validIntervals := map[string]bool{
		"1m": true, "3m": true, "5m": true, "15m": true, "30m": true,
		"1h": true, "2h": true, "4h": true, "6h": true, "8h": true, "12h": true,
		"1d": true, "3d": true, "1w": true, "1M": true,
	}
	if !validIntervals[interval] {
		return errors.New("invalid interval, must be one of: 1m, 3m, 5m, 15m, 30m, 1h, 2h, 4h, 6h, 8h, 12h, 1d, 3d, 1w, 1M")
	}
	return nil
}

// parseSymbols parses comma-separated symbols
func parseSymbols(symbolString string) ([]string, error) {
	symbolStrings := strings.Split(symbolString, ",")
	symbols := make([]string, 0, len(symbolStrings))
	for _, s := range symbolStrings {
		trimmed := strings.TrimSpace(s)
		if trimmed != "" {
			symbols = append(symbols, trimmed)
		}
	}

	if len(symbols) == 0 {
		return nil, errors.New("at least one valid symbol is required")
	}
	return symbols, nil
}

// processSymbols calculates price difference for all symbols
func (e *Executable) processSymbols(ctx context.Context, binanceIntegration binance.Integration, symbols []string, interval string, periods int) ([]*Response, error) {
	responses := make([]*Response, 0, len(symbols))
	for _, symbol := range symbols {
		response, err := e.calculatePriceDifferenceForSymbol(ctx, binanceIntegration, symbol, interval, periods)
		if err != nil {
			return nil, err
		}
		responses = append(responses, response)
	}
	return responses, nil
}

// formatResponse formats the response based on number of symbols
func formatResponse(responses []*Response) interface{} {
	// Return single response if only one symbol, array if multiple
	if len(responses) == 1 {
		return *responses[0]
	}

	// Convert to array of values instead of pointers for consistent JSON serialization
	responseValues := make([]Response, len(responses))
	for i, resp := range responses {
		responseValues[i] = *resp
	}
	return responseValues
}

func (e *Executable) Execute(ctx context.Context, modifiedConfig string) (interface{}, map[string]string, error) {
	binanceIntegration, err := e.loadIntegration(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("error loading integration: %v", err)
	}

	config, err := e.parseConfig(modifiedConfig)
	if err != nil {
		return nil, nil, err
	}

	periods, err := validateAndNormalizeConfig(&config)
	if err != nil {
		return nil, nil, err
	}

	if err := validateInterval(config.Interval); err != nil {
		return nil, nil, err
	}

	symbols, err := parseSymbols(config.Symbol)
	if err != nil {
		return nil, nil, err
	}

	responses, err := e.processSymbols(ctx, binanceIntegration, symbols, config.Interval, periods)
	if err != nil {
		return nil, nil, err
	}

	return formatResponse(responses), nil, nil
}

func NewExecutable(cfg Config) (*Executable, error) {
	if cfg.Integration == "" {
		return nil, errors.New("integration is required")
	}

	return &Executable{
		config: cfg,
	}, nil
}
