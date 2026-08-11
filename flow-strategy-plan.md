# 500-Stock × 30-Day Flow Detection Strategy

## Architecture

```
Polygon.io API (flat files or REST)
        │
        ▼  daily ETL (Python, ~2 min per stock)
        │
┌───────────────────────────────────────────┐
│  ClickHouse stocks.*                       │
│                                            │
│  daily_flow     — 1 row/stock/day          │
│  intraday_flow  — 15-min windows          │
│  signals        — buy/sell history         │
└───────────────────────────────────────────┘
        │
        ▼  cross-sectional queries
┌───────────────────────────────────────────┐
│  Signal Generator                           │
│                                            │
│  Flow Score = 0.4·Z + 3·ΔZ + 2·VolRatio   │
│  - price_momentum_penalty                  │
│                                            │
│  BUY:  Z_rising + price_flat               │
│  SELL: Z_falling_from_peak + price_up      │
└───────────────────────────────────────────┘
```

## ClickHouse Schema (`stocks.*`)

| Table | Rows | Purpose |
|-------|------|---------|
| `daily_flow` | 500 stocks × 30 days = 15,000 | One row per stock per day with all indicators |
| `intraday_flow` | 500 × 30 × 26 windows = 390,000 | 15-min Z-scores for intraday timing |
| `signals` | Variable | Historical buy/sell signals with P&L tracking |

## Indicators — Comparative Analysis

### Tridiagonal Z-Score (OURS)
- **What it measures**: Trade timing regularity → institutional vs retail flow
- **Signal**: Z > μ+2σ = structured institutional flow. Rising Z = accumulation.
- **Data needed**: Trade timestamps (ms precision)
- **Strengths**: Detects hidden order execution (iceberg, VWAP, algo slicing)
- **Weaknesses**: Requires tick-level data; doesn't capture direction

### 1. VPIN (Volume-Synchronized Probability of Informed Trading)
- **What it measures**: Volume imbalance in equal-volume buckets → informed trading
- **Signal**: VPIN > 0.8 = high probability of informed trading → adverse selection
- **Data needed**: Trade direction (buy/sell classification) + volume
- **Computational cost**: O(N log N) — bucket assignment + CDF estimation
- **vs Z-score**: VPIN detects information asymmetry; Z-score detects execution structure. Complementary.

### 2. Kyle's Lambda (Price Impact)
- **What it measures**: ΔPrice / ΔVolume → how much price moves per unit of flow
- **Signal**: Rising lambda = informed trading absorbing liquidity
- **Data needed**: OHLCV bars (1-min)
- **Computational cost**: O(N) — simple regression
- **vs Z-score**: Lambda measures PRICE response to flow; Z-score measures FLOW structure itself

### 3. Amihud Illiquidity
- **What it measures**: |Return| / DollarVolume → price impact of trading
- **Signal**: Rising illiquidity = market becoming fragile
- **Data needed**: Daily OHLCV
- **Computational cost**: O(1) per day
- **vs Z-score**: Amihud measures LIQUIDITY; Z-score measures PARTICIPANT TYPE

### 4. Trade Size Clustering
- **What it measures**: Round-lot clustering (100, 500, 1000 shares) → retail
- **Signal**: Decreasing round-lot % = institutional dominance
- **Data needed**: Individual trade sizes
- **Computational cost**: O(N) — histogram
- **vs Z-score**: Size clustering is a simpler version of what Z-score captures more subtly

### 5. Order Flow Imbalance
- **What it measures**: (BuyVol - SellVol) / TotalVol → directional pressure
- **Signal**: Persistent imbalance > 0.2 → directional informed trading
- **Data needed**: Trade direction classification (Lee-Ready)
- **Computational cost**: O(N) — tick rule classification
- **vs Z-score**: OFI measures DIRECTION; Z-score measures STRUCTURE. Combine for full picture.

### 6. VWAP Deviation
- **What it measures**: |Price - VWAP| → execution urgency
- **Signal**: Large deviation = institution willing to pay spread → urgency
- **Data needed**: Intraday prices + volumes
- **Computational cost**: O(N) — running VWAP
- **vs Z-score**: VWAP deviation shows URGENCY; Z-score shows PATTERN

## Implementation Plan

### Week 1: Data Pipeline
- [ ] Polygon flat files subscription ($199/mo Stocks Starter)
- [ ] Daily ETL: flat files → ClickHouse `stocks.daily_flow`
- [ ] Compute all 6 indicators + Z-score per stock per day

### Week 2: Backtest Framework
- [ ] Historical load: 6 months of data (90 trading days)
- [ ] Signal generation: Flow Score > threshold → BUY; < threshold → SELL
- [ ] P&L tracking in `stocks.signals`

### Week 3: Optimization
- [ ] Flow Score weights via cross-validation
- [ ] Sector rotation: long top-Z sectors, short bottom-Z sectors
- [ ] Intraday timing: use `stocks.intraday_flow` for entry/exit

### Week 4: Production
- [ ] Daily signal generation at market close
- [ ] Email/API delivery of buy/sell list
- [ ] Sharpe ratio monitoring
