package main

// @title           Ledger Backend API
// @version         1.0
// @description     E-Wallet REST API with user auth, top-up, transfer, and transaction history.
// @description     Built with Go, Gin, GORM, PostgreSQL, JWT, and bcrypt.

// @contact.name   Bintang Ridwan Pribadi
// @contact.email  bintangridwan30@gmail.com

// @host      localhost:8080
// @BasePath  /

// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Type "Bearer {token}" to authenticate.

import (
	"context"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/bntngridp/ledger-backend/internal/delivery"
	repo "github.com/bntngridp/ledger-backend/internal/repository"
	"github.com/bntngridp/ledger-backend/internal/usecase"
	"github.com/bntngridp/ledger-backend/pkg/blockchain"
	"github.com/bntngridp/ledger-backend/pkg/database"
	"github.com/bntngridp/ledger-backend/pkg/middleware"
	"github.com/bntngridp/ledger-backend/pkg/midtrans"
	"github.com/bntngridp/ledger-backend/pkg/price"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/shopspring/decimal"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"

	_ "github.com/bntngridp/ledger-backend/docs"
)

func getEnv(key, fallback string) string {
	if val, ok := os.LookupEnv(key); ok {
		return val
	}
	return fallback
}

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("no .env file found, using system env")
	}

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		log.Fatal("JWT_SECRET is required (set in .env or system env)")
	}
	expiryHoursStr := getEnv("JWT_EXPIRY_HOURS", "24")
	expiryHours, err := strconv.Atoi(expiryHoursStr)
	if err != nil {
		expiryHours = 24
	}
	port := getEnv("PORT", "8080")

	// Midtrans Configuration
	midtransServerKey := os.Getenv("MIDTRANS_SERVER_KEY")
	if midtransServerKey == "" {
		log.Fatal("MIDTRANS_SERVER_KEY is required")
	}
	midtransIsProduction := os.Getenv("MIDTRANS_IS_PRODUCTION") == "true"

	// Midtrans Iris Configuration
	irisAPIKey := os.Getenv("MIDTRANS_IRIS_API_KEY")
	irisBaseURL := getEnv("MIDTRANS_IRIS_BASE_URL", "https://app.sandbox.midtrans.com/iris")

	// Crypto Configuration
	cryptoEncryptionKeyBase64 := os.Getenv("CRYPTO_ENCRYPTION_KEY")
	if cryptoEncryptionKeyBase64 == "" {
		log.Fatal("CRYPTO_ENCRYPTION_KEY is required")
	}

	// Alchemy Configuration
	alchemyHTTPURL := os.Getenv("ALCHEMY_HTTP_URL")
	alchemyWSURL := os.Getenv("ALCHEMY_WS_URL")
	alchemyNetwork := getEnv("ALCHEMY_NETWORK", "polygon-amoy")

	// Smart Contract Addresses
	usdtContractAddress := os.Getenv("USDT_CONTRACT_ADDRESS")
	usdcContractAddress := os.Getenv("USDC_CONTRACT_ADDRESS")

	// Price Cache Configuration
	binanceAPIURL := getEnv("BINANCE_API_URL", "https://api.binance.com/api/v3")
	usdIDRRateStr := getEnv("USD_IDR_RATE", "16200")
	usdIDRRate, err := decimal.NewFromString(usdIDRRateStr)
	if err != nil {
		usdIDRRate = decimal.NewFromInt(16200)
	}

	// Swap Fee Configuration
	swapFeeStr := getEnv("SWAP_FEE_PERCENTAGE", "0.005")
	swapFee, err := decimal.NewFromString(swapFeeStr)
	if err != nil {
		swapFee = decimal.NewFromFloat(0.005)
	}

	dbCfg := database.Config{
		Host:     getEnv("DB_HOST", "localhost"),
		Port:     getEnv("DB_PORT", "5432"),
		User:     getEnv("DB_USER", "postgres"),
		Password: getEnv("DB_PASSWORD", ""),
		DBName:   getEnv("DB_NAME", "ledger-db"),
		SSLMode:  getEnv("DB_SSLMODE", "disable"),
		LogLevel: getEnv("DB_LOG_LEVEL", "warn"),
	}

	db, err := database.InitDB(dbCfg)
	if err != nil {
		log.Fatalf("database init failed: %v", err)
	}

	if err := database.RunMigrations(db); err != nil {
		log.Fatalf("migration failed: %v", err)
	}

	// Initialize Clients
	midtransClient := midtrans.NewMidtransClient(midtransServerKey, midtransIsProduction)
	irisClient := midtrans.NewIrisClient(irisAPIKey, irisBaseURL)
	alchemyClient := blockchain.NewAlchemyClient(alchemyHTTPURL, alchemyWSURL)
	priceCache := price.NewPriceCache(binanceAPIURL, usdIDRRate)

	// Initialize Repositories
	userRepo := repo.NewUserRepository(db)
	walletRepo := repo.NewWalletRepository(db)
	txRepo := repo.NewTransactionRepository(db)
	cryptoAddrRepo := repo.NewCryptoAddressRepository(db)

	// Initialize ERC-20 Listener (Goroutine)
	contractAssets := make(map[string]string)
	contractDecimals := make(map[string]int)
	if usdtContractAddress != "" {
		contractAssets[strings.ToLower(usdtContractAddress)] = "USDT"
		contractDecimals[strings.ToLower(usdtContractAddress)] = 6
	}
	if usdcContractAddress != "" {
		contractAssets[strings.ToLower(usdcContractAddress)] = "USDC"
		contractDecimals[strings.ToLower(usdcContractAddress)] = 6
	}

	listenerDeps := blockchain.ListenerDeps{
		AlchemyClient:     alchemyClient,
		CryptoAddressRepo: cryptoAddrRepo,
		TransactionRepo:   txRepo,
		ContractAssets:    contractAssets,
		ContractDecimals:  contractDecimals,
		Network:           alchemyNetwork,
	}
	erc20Listener := blockchain.NewERC20Listener(listenerDeps)

	// Start On-chain event listener in background context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if alchemyWSURL != "" && !strings.Contains(alchemyWSURL, "your-api-key") && len(contractAssets) > 0 {
		go erc20Listener.Start(ctx)
	} else {
		log.Println("[WARN] Alchemy WebSocket URL is placeholder or missing contract addresses, On-chain listener is disabled")
	}

	// Initialize Usecases
	authUC := usecase.NewAuthUsecase(userRepo, walletRepo)
	transferUC := usecase.NewTransferUsecase(walletRepo, txRepo)
	walletUC := usecase.NewWalletUsecase(walletRepo, txRepo, midtransClient, priceCache)
	webhookUC := usecase.NewWebhookUsecase(txRepo, midtransClient)

	contractAddrs := map[string]string{
		"polygon_amoy_USDT": usdtContractAddress,
		"polygon_amoy_USDC": usdcContractAddress,
		"sepolia_USDT":      usdtContractAddress,
		"sepolia_USDC":      usdcContractAddress,
	}

	cryptoUC, err := usecase.NewCryptoUsecase(usecase.CryptoUsecaseConfig{
		WalletRepo:          walletRepo,
		TxRepo:              txRepo,
		CryptoAddrRepo:      cryptoAddrRepo,
		EncryptionKeyBase64: cryptoEncryptionKeyBase64,
		AlchemyClient:       alchemyClient,
		ContractAddrs:       contractAddrs,
		Listener:            erc20Listener,
	})
	if err != nil {
		log.Fatalf("failed to initialize crypto usecase: %v", err)
	}

	exchangeUC := usecase.NewExchangeUsecase(walletRepo, txRepo, priceCache, swapFee)
	fiatUC := usecase.NewFiatUsecase(walletRepo, txRepo, irisClient)

	// Initialize Handlers
	authHandler := delivery.NewAuthHandler(authUC, jwtSecret, expiryHours)
	transferHandler := delivery.NewTransferHandler(transferUC)
	walletHandler := delivery.NewWalletHandler(walletUC)
	webhookHandler := delivery.NewWebhookHandler(webhookUC)
	cryptoHandler := delivery.NewCryptoHandler(cryptoUC)
	exchangeHandler := delivery.NewExchangeHandler(exchangeUC)
	fiatHandler := delivery.NewFiatHandler(fiatUC)

	googleConfig := &oauth2.Config{
		ClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
		ClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
		RedirectURL:  os.Getenv("GOOGLE_REDIRECT_URL"),
		Scopes: []string{
			"https://www.googleapis.com/auth/userinfo.profile",
			"https://www.googleapis.com/auth/userinfo.email",
		},
		Endpoint: google.Endpoint,
	}
	oauthHandler := delivery.NewOAuthHandler(authUC, googleConfig, jwtSecret, expiryHours)

	r := gin.Default()

	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	r.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "Ledger Backend API",
			"docs":    "/swagger/index.html",
		})
	})

	api := r.Group("/api/v1")
	{
		auth := api.Group("/auth")
		{
			auth.POST("/register", authHandler.Register)
			auth.POST("/login", authHandler.Login)
			auth.GET("/google", oauthHandler.LoginGoogle)
			auth.GET("/google/callback", oauthHandler.GoogleCallback)
		}

		// Public Webhook route for Midtrans notification callbacks
		api.POST("/webhooks/midtrans", webhookHandler.HandleMidtrans)

		api.Use(middleware.JWTAuth(jwtSecret))
		{
			api.POST("/transfer", transferHandler.Transfer)
			api.POST("/topup", walletHandler.TopUp)
			api.GET("/transactions", walletHandler.GetTransactionHistory)
			api.GET("/wallet/dashboard", walletHandler.GetDashboard)

			// Crypto routes
			api.GET("/crypto/address", cryptoHandler.GetDepositAddress)
			api.POST("/crypto/withdraw", cryptoHandler.WithdrawCrypto)

			// Exchange routes
			api.GET("/exchange/rate", exchangeHandler.GetRate)
			api.POST("/exchange/swap", exchangeHandler.Swap)

			// Fiat withdrawal route
			api.POST("/fiat/withdraw", fiatHandler.WithdrawFiat)
		}
	}

	r.GET("/ping", middleware.JWTAuth(jwtSecret), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "pong",
		})
	})

	log.Printf("server starting on port %s", port)
	log.Printf("swagger docs: http://localhost:%s/swagger/index.html", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
