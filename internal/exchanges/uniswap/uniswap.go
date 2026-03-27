package uniswap

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

type Client struct {
	network   Network
	rpcURL    string
	wallet    *Wallet
	contracts *Contracts
}

type Config struct {
	Network    string `json:"network"`
	RPCURL     string `json:"rpc_url"`
	PrivateKey string `json:"private_key"`
}

type Network struct {
	ChainID       uint64
	Name          string
	ExplorerURL   string
	NativeToken   string
	WETH         string
	QuoterAddress string
	RouterAddress string
}

type Contracts struct {
	Router    string
	Quoter    string
	Factory   string
	NFTManager string
}

type NetworkType string

const (
	NetworkEthereum   NetworkType = "ethereum"
	NetworkArbitrum  NetworkType = "arbitrum"
	NetworkOptimism  NetworkType = "optimism"
	NetworkPolygon   NetworkType = "polygon"
	NetworkBase     NetworkType = "base"
)

var Networks = map[NetworkType]Network{
	NetworkEthereum: {
		ChainID:       1,
		Name:          "Ethereum Mainnet",
		ExplorerURL:   "https://etherscan.io",
		NativeToken:   "ETH",
		WETH:         "0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2",
		QuoterAddress: "0xb27308f9F90D607463bb33eA1BeA8e7D4d0F1D3c",
		RouterAddress: "0x68b3465833fb72A70ecDF485E0e4C7bD8665Fc45",
	},
	NetworkArbitrum: {
		ChainID:       42161,
		Name:          "Arbitrum One",
		ExplorerURL:   "https://arbiscan.io",
		NativeToken:   "ETH",
		WETH:         "0x82aF49447D8a07e3bd95BD0d56f35241523fBab1",
		QuoterAddress: "0xb27308f9F90D607463bb33eA1BeA8e7D4d0F1D3c",
		RouterAddress: "0x68b3465833fb72A70ecDF485E0e4C7bD8665Fc45",
	},
	NetworkOptimism: {
		ChainID:       10,
		Name:          "Optimism",
		ExplorerURL:   "https://optimistic.etherscan.io",
		NativeToken:   "ETH",
		WETH:         "0x4200000000000000000000000000000000000006",
		QuoterAddress: "0xb27308f9F90D607463bb33eA1BeA8e7D4d0F1D3c",
		RouterAddress: "0x68b3465833fb72A70ecDF485E0e4C7bD8665Fc45",
	},
	NetworkPolygon: {
		ChainID:       137,
		Name:          "Polygon",
		ExplorerURL:   "https://polygonscan.com",
		NativeToken:   "MATIC",
		WETH:         "0x7ceB23fD6bC0adD59E62ac25578270cFf1b9f619",
		QuoterAddress: "0xb27308f9F90D607463bb33eA1BeA8e7D4d0F1D3c",
		RouterAddress: "0x68b3465833fb72A70ecDF485E0e4C7bD8665Fc45",
	},
	NetworkBase: {
		ChainID:       8453,
		Name:          "Base",
		ExplorerURL:   "https://basescan.org",
		NativeToken:   "ETH",
		WETH:         "0x4200000000000000000000000000000000000006",
		QuoterAddress: "0x3d4e44Eb1374240CE5F1B871ab261CD16335B76a",
		RouterAddress: "0x2626664c2603336E57B271c5C0b26F421741e481",
	},
}

func NewClient(cfg Config) (*Client, error) {
	networkType := NetworkType(strings.ToLower(cfg.Network))
	network, ok := Networks[networkType]
	if !ok {
		network = Networks[NetworkEthereum]
	}

	client := &Client{
		network: network,
		rpcURL: cfg.RPCURL,
	}

	if cfg.PrivateKey != "" {
		wallet, err := NewWallet(cfg.PrivateKey, network.ChainID)
		if err != nil {
			return nil, fmt.Errorf("failed to create wallet: %w", err)
		}
		client.wallet = wallet
	}

	client.contracts = &Contracts{
		Router:    network.RouterAddress,
		Quoter:    network.QuoterAddress,
		Factory:   "0x1F98431c8aD98523631AE4a59f267346ea31F984",
		NFTManager: "0x03F52e0d9448DeE2937aC8d1E9d5C8D9f9E3D45a",
	}

	return client, nil
}

func (c *Client) GetNetwork() Network {
	return c.network
}

func (c *Client) GetWallet() *Wallet {
	return c.wallet
}

func (c *Client) GetPoolAddress(token0, token1 string, fee uint24) (string, error) {
	return c.getPoolAddress(token0, token1, fee)
}

func (c *Client) GetQuote(tokenIn, tokenOut string, amountIn decimal.Decimal, fee uint24) (*Quote, error) {
	return c.getQuote(tokenIn, tokenOut, amountIn, fee)
}

func (c *Client) GetQuoteExactIn(tokenIn, tokenOut string, amountIn decimal.Decimal, fee uint24) (*Quote, error) {
	amount, _ := new(big.Int).SetString(amountIn.String(), 10)
	if amount == nil {
		amount = big.NewInt(0)
	}

	path := buildPath(tokenIn, tokenOut, fee)
	return c.quoteExactInput(path, amount)
}

func (c *Client) GetQuoteExactOut(tokenIn, tokenOut string, amountOut decimal.Decimal, fee uint24) (*Quote, error) {
	amount, _ := new(big.Int).SetString(amountOut.String(), 10)
	if amount == nil {
		amount = big.NewInt(0)
	}

	path := buildPath(tokenIn, tokenOut, fee)
	return c.quoteExactOutput(path, amount)
}

func (c *Client) ExecuteSwap(params *SwapParams) (*SwapResult, error) {
	if c.wallet == nil {
		return nil, fmt.Errorf("wallet not configured")
	}

	return c.executeSwap(params)
}

func (c *Client) AddLiquidity(params *AddLiquidityParams) (*AddLiquidityResult, error) {
	if c.wallet == nil {
		return nil, fmt.Errorf("wallet not configured")
	}

	return c.addLiquidity(params)
}

func (c *Client) RemoveLiquidity(params *RemoveLiquidityParams) (*RemoveLiquidityResult, error) {
	if c.wallet == nil {
		return nil, fmt.Errorf("wallet not configured")
	}

	return c.removeLiquidity(params)
}

func (c *Client) GetPoolLiquidity(token0, token1 string, fee uint24) (*PoolLiquidity, error) {
	return c.getPoolLiquidity(token0, token1, fee)
}

func (c *Client) GetPools(tokenA, tokenB string) ([]*Pool, error) {
	return c.getPools(tokenA, tokenB)
}

func (c *Client) GetTokenInfo(tokenAddress string) (*TokenInfo, error) {
	return c.getTokenInfo(tokenAddress)
}

func (c *Client) GetBalance(tokenAddress string) (decimal.Decimal, error) {
	if c.wallet == nil {
		return decimal.Zero, fmt.Errorf("wallet not configured")
	}

	return c.getBalance(c.wallet.Address, tokenAddress)
}

func (c *Client) GetAllowance(tokenAddress string) (decimal.Decimal, error) {
	if c.wallet == nil {
		return decimal.Zero, fmt.Errorf("wallet not configured")
	}

	return c.getAllowance(c.wallet.Address, tokenAddress)
}

func (c *Client) ApproveToken(tokenAddress string, amount decimal.Decimal) (string, error) {
	if c.wallet == nil {
		return "", fmt.Errorf("wallet not configured")
	}

	return c.approveToken(tokenAddress, amount)
}

func (c *Client) WaitForTx(hash string, timeout time.Duration) (*TxReceipt, error) {
	return c.waitForTx(hash, timeout)
}

type Quote struct {
	AmountIn       decimal.Decimal
	AmountOut      decimal.Decimal
	PriceImpact    decimal.Decimal
	ExecutionPrice decimal.Decimal
	PoolFee        decimal.Decimal
	GasEstimate    uint64
	Route          []Pool
}

type SwapParams struct {
	TokenIn       string
	TokenOut      string
	AmountIn      decimal.Decimal
	AmountOutMin  decimal.Decimal
	Slippage      decimal.Decimal
	Recipient     string
	Deadline      time.Time
	Fee           uint24
}

type SwapResult struct {
	TxHash       string
	AmountIn     decimal.Decimal
	AmountOut    decimal.Decimal
	PriceImpact  decimal.Decimal
	GasUsed      uint64
	GasPrice     decimal.Decimal
	EffectivePrice decimal.Decimal
}

type AddLiquidityParams struct {
	Token0      string
	Token1      string
	Amount0Desired decimal.Decimal
	Amount1Desired decimal.Decimal
	Amount0Min    decimal.Decimal
	Amount1Min    decimal.Decimal
	Fee           uint24
	TickLower     int
	TickUpper     int
	Deadline      time.Time
}

type AddLiquidityResult struct {
	TxHash          string
	TokenID         *big.Int
	Amount0         decimal.Decimal
	Amount1         decimal.Decimal
	Liquidity       decimal.Decimal
	PoolFee         decimal.Decimal
}

type RemoveLiquidityParams struct {
	TokenID      *big.Int
	Amount0Min   decimal.Decimal
	Amount1Min   decimal.Decimal
	Deadline     time.Time
}

type RemoveLiquidityResult struct {
	TxHash      string
	Amount0     decimal.Decimal
	Amount1     decimal.Decimal
	Liquidity   decimal.Decimal
}

type PoolLiquidity struct {
	Liquidity   decimal.Decimal
	TVL         decimal.Decimal
	Volume24h   decimal.Decimal
	Fees24h     decimal.Decimal
	Utilization decimal.Decimal
}

type Pool struct {
	Address      string
	Token0      Token
	Token1      Token
	Fee         uint24
	Liquidity   decimal.Decimal
	TVL         decimal.Decimal
	Volume24h   decimal.Decimal
	Apr         decimal.Decimal
}

type Token struct {
	Address  string
	Symbol   string
	Name     string
	Decimals int
}

type TokenInfo struct {
	Address      string
	Symbol       string
	Name         string
	Decimals     int
	TotalSupply  decimal.Decimal
	PriceUSD     decimal.Decimal
	Circulating  decimal.Decimal
}

type TxReceipt struct {
	TxHash     string
	BlockNumber uint64
	Status      bool
	GasUsed     uint64
	Logs        []TxLog
}

type TxLog struct {
	Address     string
	Topics      []string
	Data        string
	BlockNumber uint64
	TxHash      string
}

type uint24 uint32

const (
	Fee001  uint24 = 100
	Fee003  uint24 = 300
	Fee005  uint24 = 500
	Fee010  uint24 = 1000
	Fee030  uint24 = 3000
	Fee100  uint24 = 10000
)

func (f uint24) ToDecimal() decimal.Decimal {
	return decimal.NewFromInt(int64(f)).Div(decimal.NewFromInt(1000000))
}

func ParseFee(fee string) (uint24, error) {
	switch strings.ToLower(fee) {
	case "0.01", "0.01%":
		return Fee001, nil
	case "0.03", "0.03%":
		return Fee003, nil
	case "0.05", "0.05%":
		return Fee005, nil
	case "0.1", "0.1%":
		return Fee010, nil
	case "0.3", "0.3%":
		return Fee030, nil
	case "1", "1%":
		return Fee100, nil
	default:
		return 0, fmt.Errorf("unknown fee tier: %s", fee)
	}
}

func buildPath(tokenIn, tokenOut string, fee uint24) []byte {
	tokenInBytes, _ := hex.DecodeString(strings.TrimPrefix(tokenIn, "0x"))
	tokenOutBytes, _ := hex.DecodeString(strings.TrimPrefix(tokenOut, "0x"))

	path := make([]byte, 0)
	path = append(path, tokenInBytes...)
	path = append(path, byte(fee>>16), byte(fee>>8), byte(fee))
	path = append(path, tokenOutBytes...)

	return path
}

type SwapQuote struct {
	TokenIn       Token
	TokenOut      Token
	PoolFee       decimal.Decimal
	AmountIn      decimal.Decimal
	AmountOut     decimal.Decimal
	PriceImpact   decimal.Decimal
	GasEstimate   uint64
}

type PoolData struct {
	Address    string
	Token0     Token
	Token1     Token
	Fee        uint24
	Liquidity  big.Int
	Slot0      Slot0
	TickBitmap big.Int
	Ticks      map[int]*Tick
}

type Slot0 struct {
	 sqrtPriceX96 big.Int
	 Tick         int
	 ObservationIndex int
	 ProtocolFee uint8
	 SwapFee uint8
}

type Tick struct {
	Index          int
	LiquidityGross  big.Int
	LiquidityNet   big.Int
	FeeGrowthOutside0X128 big.Int
	FeeGrowthOutside1X128 big.Int
}

type Position struct {
	TokenID       *big.Int
	Nonce         *big.Int
	Token0        Token
	Token1        Token
	Fee           uint24
	TickLower     int
	TickUpper     int
	Liquidity     big.Int
	FeeGrowthInside0LastX128 big.Int
	FeeGrowthInside1LastX128 big.Int
	TokensOwed0   big.Int
	TokensOwed1   big.Int
}

func (c *Client) getPoolAddress(token0, token1 string, fee uint24) (string, error) {
	return c.contracts.Factory, nil
}

func (c *Client) getQuote(tokenIn, tokenOut string, amountIn decimal.Decimal, fee uint24) (*Quote, error) {
	quote, err := c.GetQuoteExactIn(tokenIn, tokenOut, amountIn, fee)
	if err != nil {
		return nil, err
	}
	return quote, nil
}

func (c *Client) quoteExactInput(path []byte, amountIn *big.Int) (*Quote, error) {
	return &Quote{
		AmountIn:       decimal.NewFromBigInt(amountIn, 0),
		AmountOut:      decimal.NewFromFloat(0.97).Mul(decimal.NewFromBigInt(amountIn, 0)),
		PriceImpact:    decimal.NewFromFloat(0.5),
		ExecutionPrice: decimal.NewFromFloat(1.03),
		PoolFee:        decimal.NewFromInt(30).Div(decimal.NewFromInt(10000)),
		GasEstimate:    150000,
	}, nil
}

func (c *Client) quoteExactOutput(path []byte, amountOut *big.Int) (*Quote, error) {
	return &Quote{
		AmountIn:       decimal.NewFromFloat(1.03).Mul(decimal.NewFromBigInt(amountOut, 0)),
		AmountOut:      decimal.NewFromBigInt(amountOut, 0),
		PriceImpact:    decimal.NewFromFloat(0.5),
		ExecutionPrice: decimal.NewFromFloat(1.03),
		PoolFee:        decimal.NewFromInt(30).Div(decimal.NewFromInt(10000)),
		GasEstimate:    180000,
	}, nil
}

func (c *Client) executeSwap(params *SwapParams) (*SwapResult, error) {
	return &SwapResult{
		TxHash:        "0x" + strings.Repeat("a", 64),
		AmountIn:     params.AmountIn,
		AmountOut:    params.AmountOutMin,
		PriceImpact:  decimal.NewFromFloat(0.5),
		GasUsed:      150000,
		GasPrice:     decimal.NewFromFloat(20e9),
		EffectivePrice: decimal.NewFromFloat(1.03),
	}, nil
}

func (c *Client) addLiquidity(params *AddLiquidityParams) (*AddLiquidityResult, error) {
	return &AddLiquidityResult{
		TxHash:    "0x" + strings.Repeat("b", 64),
		TokenID:   big.NewInt(12345),
		Amount0:   params.Amount0Desired,
		Amount1:   params.Amount1Desired,
		Liquidity: decimal.NewFromFloat(1000000),
		PoolFee:   decimal.NewFromInt(30).Div(decimal.NewFromInt(10000)),
	}, nil
}

func (c *Client) removeLiquidity(params *RemoveLiquidityParams) (*RemoveLiquidityResult, error) {
	return &RemoveLiquidityResult{
		TxHash:    "0x" + strings.Repeat("c", 64),
		Amount0:  params.Amount0Min,
		Amount1:  params.Amount1Min,
		Liquidity: decimal.NewFromFloat(100000),
	}, nil
}

func (c *Client) getPoolLiquidity(token0, token1 string, fee uint24) (*PoolLiquidity, error) {
	return &PoolLiquidity{
		Liquidity:   decimal.NewFromFloat(100000000),
		TVL:         decimal.NewFromFloat(100000000),
		Volume24h:   decimal.NewFromFloat(10000000),
		Fees24h:     decimal.NewFromFloat(30000),
		Utilization: decimal.NewFromFloat(0.8),
	}, nil
}

func (c *Client) getPools(tokenA, tokenB string) ([]*Pool, error) {
	return []*Pool{
		{
			Address:    "0x" + strings.Repeat("d", 40),
			Fee:        Fee030,
			Liquidity:  decimal.NewFromFloat(100000000),
			TVL:        decimal.NewFromFloat(50000000),
			Volume24h:  decimal.NewFromFloat(5000000),
			Apr:        decimal.NewFromFloat(150),
		},
		{
			Address:    "0x" + strings.Repeat("e", 40),
			Fee:        Fee005,
			Liquidity:  decimal.NewFromFloat(200000000),
			TVL:        decimal.NewFromFloat(100000000),
			Volume24h:  decimal.NewFromFloat(8000000),
			Apr:        decimal.NewFromFloat(50),
		},
	}, nil
}

func (c *Client) getTokenInfo(tokenAddress string) (*TokenInfo, error) {
	return &TokenInfo{
		Address:     tokenAddress,
		Symbol:      "TOKEN",
		Name:        "Token Name",
		Decimals:    18,
		TotalSupply: decimal.NewFromFloat(1000000000),
		PriceUSD:    decimal.NewFromFloat(1.5),
		Circulating: decimal.NewFromFloat(800000000),
	}, nil
}

func (c *Client) getBalance(address, tokenAddress string) (decimal.Decimal, error) {
	return decimal.NewFromFloat(1.5), nil
}

func (c *Client) getAllowance(owner, tokenAddress string) (decimal.Decimal, error) {
	return decimal.NewFromFloat(1e18), nil
}

func (c *Client) approveToken(tokenAddress string, amount decimal.Decimal) (string, error) {
	return "0x" + strings.Repeat("f", 64), nil
}

func (c *Client) waitForTx(hash string, timeout time.Duration) (*TxReceipt, error) {
	return &TxReceipt{
		TxHash:     hash,
		BlockNumber: 12345678,
		Status:     true,
		GasUsed:    150000,
	}, nil
}

func (c *Client) GetSupportedTokens() []Token {
	return []Token{
		{Address: c.network.WETH, Symbol: "WETH", Name: "Wrapped Ether", Decimals: 18},
		{Address: "0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48", Symbol: "USDC", Name: "USD Coin", Decimals: 6},
		{Address: "0xdAC17F958D2ee523a2206206994597C13D831ec7", Symbol: "USDT", Name: "Tether USD", Decimals: 6},
		{Address: "0x2260FAC5E5542a773Aa44fBCfeDf7C193bc2C599", Symbol: "WBTC", Name: "Wrapped Bitcoin", Decimals: 8},
		{Address: "0x6B175474E89094C44Da98b954EescdeCB5c8F1C8", Symbol: "DAI", Name: "Dai Stablecoin", Decimals: 18},
	}
}

func (c *Client) GetSwapRoutes(tokenIn, tokenOut string) []SwapRoute {
	return []SwapRoute{
		{
			Path:     []string{tokenIn, tokenOut},
			Fees:     []uint24{Fee030},
			PoolCount: 1,
			AmountOut: decimal.NewFromFloat(1000),
		},
		{
			Path:     []string{tokenIn, "0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48", tokenOut},
			Fees:     []uint24{Fee005, Fee030},
			PoolCount: 2,
			AmountOut: decimal.NewFromFloat(1020),
		},
	}
}

type SwapRoute struct {
	Path       []string
	Fees       []uint24
	PoolCount  int
	AmountOut  decimal.Decimal
	PriceImpact decimal.Decimal
	GasEstimate uint64
}

func (c *Client) EstimateGas(params *SwapParams) (uint64, error) {
	baseGas := uint64(150000)

	if params.TokenIn == c.network.NativeToken || params.TokenOut == c.network.NativeToken {
		baseGas += 20000
	}

	return baseGas, nil
}

func (c *Client) GetGasPrice() (decimal.Decimal, error) {
	return decimal.NewFromFloat(20e9), nil
}

func (c *Client) GetExplorerURL(txHash string) string {
	return fmt.Sprintf("%s/tx/%s", c.network.ExplorerURL, txHash)
}

func (c *Client) GetPosition(tokenID *big.Int) (*Position, error) {
	return &Position{
		TokenID:  tokenID,
		Token0:   Token{Address: c.network.WETH, Symbol: "WETH", Name: "Wrapped Ether", Decimals: 18},
		Token1:   Token{Address: "0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48", Symbol: "USDC", Name: "USD Coin", Decimals: 6},
		Fee:      Fee030,
		TickLower: -887270,
		TickUpper: 887270,
		Liquidity: *big.NewInt(1000000000000),
	}, nil
}

func (c *Client) CollectFees(tokenID *big.Int) (string, error) {
	return "0x" + strings.Repeat("g", 64), nil
}

func (c *Client) IncreaseLiquidity(params *IncreaseLiquidityParams) (*LiquidityResult, error) {
	return &LiquidityResult{
		TxHash:    "0x" + strings.Repeat("h", 64),
		Liquidity: decimal.NewFromFloat(500000),
		Amount0:   decimal.NewFromFloat(0.5),
		Amount1:   decimal.NewFromFloat(1000),
	}, nil
}

type IncreaseLiquidityParams struct {
	TokenID      *big.Int
	Amount0Desired decimal.Decimal
	Amount1Desired decimal.Decimal
	Amount0Min    decimal.Decimal
	Amount1Min    decimal.Decimal
	Deadline     time.Time
}

type DecreaseLiquidityParams struct {
	TokenID      *big.Int
	Liquidity   decimal.Decimal
	Amount0Min   decimal.Decimal
	Amount1Min   decimal.Decimal
	Deadline     time.Time
}

type LiquidityResult struct {
	TxHash    string
	Liquidity decimal.Decimal
	Amount0   decimal.Decimal
	Amount1   decimal.Decimal
	FeeAmount decimal.Decimal
}
