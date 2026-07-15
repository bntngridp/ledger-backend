// Package price provides a thread-safe in-memory cache for exchange rates fetched from
// the Binance Public API. The cache refreshes automatically when TTL expires.
package price

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/shopspring/decimal"
)

const cacheTTL = 30 * time.Second

// CachedRate holds a rate value alongside the time it was fetched.
type CachedRate struct {
	Rate      decimal.Decimal
	FetchedAt time.Time
}

// PriceCache fetches and caches exchange rates from the Binance Public API.
// All methods are safe for concurrent use.
type PriceCache struct {
	mu         sync.RWMutex
	cache      map[string]CachedRate
	binanceURL string
	usdIDRRate decimal.Decimal  // Fallback / configured USD→IDR rate
	httpClient *http.Client
}

// NewPriceCache creates a new PriceCache.
//   - binanceURL: e.g., "https://api.binance.com/api/v3"
//   - usdIDRRate: fallback rate, e.g., 16200 (can be updated via env)
func NewPriceCache(binanceURL string, usdIDRRate decimal.Decimal) *PriceCache {
	return &PriceCache{
		cache:      make(map[string]CachedRate),
		binanceURL: binanceURL,
		usdIDRRate: usdIDRRate,
		httpClient: &http.Client{Timeout: 5 * time.Second},
	}
}

// GetRate returns the exchange rate for the given pair (e.g., "USDT_IDR", "USDC_IDR").
// If the cached value has expired, it fetches a fresh rate from Binance.
func (p *PriceCache) GetRate(pair string) (decimal.Decimal, time.Time, error) {
	p.mu.RLock()
	if cached, ok := p.cache[pair]; ok && time.Since(cached.FetchedAt) < cacheTTL {
		p.mu.RUnlock()
		return cached.Rate, cached.FetchedAt, nil
	}
	p.mu.RUnlock()

	// Cache miss or expired — fetch fresh rate.
	rate, err := p.fetchRate(pair)
	if err != nil {
		// Return stale data if available, rather than failing hard.
		p.mu.RLock()
		if cached, ok := p.cache[pair]; ok {
			p.mu.RUnlock()
			return cached.Rate, cached.FetchedAt, nil
		}
		p.mu.RUnlock()
		return decimal.Zero, time.Time{}, fmt.Errorf("rate unavailable for %s: %w", pair, err)
	}

	now := time.Now()
	p.mu.Lock()
	p.cache[pair] = CachedRate{Rate: rate, FetchedAt: now}
	p.mu.Unlock()

	return rate, now, nil
}

// fetchRate resolves a pair to a Binance symbol and fetches the price.
// Strategy:
// fetchRate resolves a pair to a Binance symbol and fetches the price.
// Strategy:
//   - USDT_IDR: Assume 1 USDT = 1 USD (1.0) * USD_IDR_RATE (no USDTUSDT pair on Binance)
//   - USDC_IDR: Fetch USDC/USDT price from Binance * USD_IDR_RATE
func (p *PriceCache) fetchRate(pair string) (decimal.Decimal, error) {
	if pair == "USDT_IDR" {
		return p.usdIDRRate, nil
	}

	if pair == "USDC_IDR" {
		usdPrice, err := p.fetchBinancePrice("USDCUSDT")
		if err != nil {
			// Fallback to 1.0 if Binance is down
			return p.usdIDRRate, nil
		}
		return usdPrice.Mul(p.usdIDRRate), nil
	}

	return decimal.Zero, fmt.Errorf("unsupported pair: %s", pair)
}

type binanceTickerResponse struct {
	Symbol string `json:"symbol"`
	Price  string `json:"price"`
}

func (p *PriceCache) fetchBinancePrice(symbol string) (decimal.Decimal, error) {
	url := fmt.Sprintf("%s/ticker/price?symbol=%s", p.binanceURL, symbol)

	resp, err := p.httpClient.Get(url)
	if err != nil {
		return decimal.Zero, fmt.Errorf("binance request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return decimal.Zero, fmt.Errorf("binance returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return decimal.Zero, fmt.Errorf("failed to read binance response: %w", err)
	}

	var ticker binanceTickerResponse
	if err := json.Unmarshal(body, &ticker); err != nil {
		return decimal.Zero, fmt.Errorf("failed to parse binance response: %w", err)
	}

	price, err := decimal.NewFromString(ticker.Price)
	if err != nil {
		return decimal.Zero, fmt.Errorf("invalid price value from binance: %w", err)
	}

	return price, nil
}
