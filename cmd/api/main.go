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
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"reflect"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/bntngridp/ledger-backend/internal/delivery"
	repo "github.com/bntngridp/ledger-backend/internal/repository"
	"github.com/bntngridp/ledger-backend/internal/usecase"
	"github.com/bntngridp/ledger-backend/pkg/blockchain"
	"github.com/bntngridp/ledger-backend/pkg/database"
	"github.com/bntngridp/ledger-backend/pkg/email"
	"github.com/bntngridp/ledger-backend/pkg/middleware"
	"github.com/bntngridp/ledger-backend/pkg/midtrans"
	"github.com/bntngridp/ledger-backend/pkg/price"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
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
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	// Register custom validator for decimal.Decimal to support validation tags like gt=0
	if v, ok := binding.Validator.Engine().(*validator.Validate); ok {
		v.RegisterCustomTypeFunc(func(field reflect.Value) interface{} {
			if val, ok := field.Interface().(decimal.Decimal); ok {
				return val.InexactFloat64()
			}
			return nil
		}, decimal.Decimal{})
	}

	if err := godotenv.Load(); err != nil {
		slog.Info("no .env file found, using system env")
	}

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		slog.Error("JWT_SECRET is required")
		os.Exit(1)
	}
	expiryHoursStr := getEnv("JWT_EXPIRY_HOURS", "24")
	expiryHours, err := strconv.Atoi(expiryHoursStr)
	if err != nil {
		expiryHours = 24
	}
	port := getEnv("PORT", "8080")

	midtransServerKey := os.Getenv("MIDTRANS_SERVER_KEY")
	if midtransServerKey == "" {
		slog.Error("MIDTRANS_SERVER_KEY is required")
		os.Exit(1)
	}
	midtransIsProduction := os.Getenv("MIDTRANS_IS_PRODUCTION") == "true"

	snapClientID := os.Getenv("MIDTRANS_SNAP_CLIENT_ID")
	snapClientSecret := os.Getenv("MIDTRANS_SNAP_CLIENT_SECRET")
	snapPartnerID := os.Getenv("MIDTRANS_SNAP_PARTNER_ID")
	snapPrivateKeyPath := getEnv("MIDTRANS_SNAP_PRIVATE_KEY_PATH", "certs/private-key.pem")
	snapBaseURL := getEnv("MIDTRANS_SNAP_BASE_URL", "https://merchants.sbx.midtrans.com")

	cryptoEncryptionKeyBase64 := os.Getenv("CRYPTO_ENCRYPTION_KEY")
	if cryptoEncryptionKeyBase64 == "" {
		slog.Error("CRYPTO_ENCRYPTION_KEY is required")
		os.Exit(1)
	}

	alchemyHTTPURL := os.Getenv("ALCHEMY_HTTP_URL")
	alchemyWSURL := os.Getenv("ALCHEMY_WS_URL")
	alchemyNetwork := getEnv("ALCHEMY_NETWORK", "polygon-amoy")

	usdtContractAddress := os.Getenv("USDT_CONTRACT_ADDRESS")
	usdcContractAddress := os.Getenv("USDC_CONTRACT_ADDRESS")

	binanceAPIURL := getEnv("BINANCE_API_URL", "https://api.binance.com/api/v3")
	usdIDRRateStr := getEnv("USD_IDR_RATE", "16200")
	usdIDRRate, err := decimal.NewFromString(usdIDRRateStr)
	if err != nil {
		usdIDRRate = decimal.NewFromInt(16200)
	}

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
		slog.Error("database init failed", "error", err)
		os.Exit(1)
	}

	if err := database.RunMigrations(db); err != nil {
		slog.Error("migration failed", "error", err)
		os.Exit(1)
	}

	midtransClient := midtrans.NewMidtransClient(midtransServerKey, midtransIsProduction)
	irisClient := midtrans.NewIrisClient(midtrans.BIConfig{
		ClientID:       snapClientID,
		ClientSecret:   snapClientSecret,
		PartnerID:      snapPartnerID,
		PrivateKeyPath: snapPrivateKeyPath,
		BaseURL:        snapBaseURL,
	})
	alchemyClient := blockchain.NewAlchemyClient(alchemyHTTPURL, alchemyWSURL)
	priceCache := price.NewPriceCache(binanceAPIURL, usdIDRRate)
	smtpHost := getEnv("SMTP_HOST", "smtp.gmail.com")
	smtpPortStr := getEnv("SMTP_PORT", "587")
	smtpPort := email.ParseSMTPPort(smtpPortStr)
	smtpUser := os.Getenv("SMTP_USER")
	smtpPassword := os.Getenv("SMTP_PASSWORD")
	smtpSenderName := getEnv("SMTP_SENDER_NAME", "Ledger Wallet")

	emailService := email.NewEmailService(smtpHost, smtpPort, smtpUser, smtpPassword, smtpSenderName, logger)

	userRepo := repo.NewUserRepository(db)
	walletRepo := repo.NewWalletRepository(db)
	txRepo := repo.NewTransactionRepository(db)
	cryptoAddrRepo := repo.NewCryptoAddressRepository(db)
	notifRepo := repo.NewNotificationRepository(db)

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

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if alchemyWSURL != "" && !strings.Contains(alchemyWSURL, "your-api-key") && len(contractAssets) > 0 {
		go erc20Listener.Start(ctx)
	} else {
		slog.Warn("Alchemy WS URL placeholder or missing contract addresses; listener disabled")
	}

	authUC := usecase.NewAuthUsecase(userRepo, walletRepo, emailService, cryptoEncryptionKeyBase64)
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
		NotifRepo:           notifRepo,
		EncryptionKeyBase64: cryptoEncryptionKeyBase64,
		AlchemyClient:       alchemyClient,
		ContractAddrs:       contractAddrs,
		Listener:            erc20Listener,
	})
	if err != nil {
		slog.Error("failed to initialize crypto usecase", "error", err)
		os.Exit(1)
	}

	exchangeUC := usecase.NewExchangeUsecase(walletRepo, txRepo, priceCache, swapFee)
	fiatUC := usecase.NewFiatUsecase(walletRepo, txRepo, irisClient)
	notifUC := usecase.NewNotificationUsecase(notifRepo)

	authHandler := delivery.NewAuthHandler(authUC, jwtSecret, expiryHours)
	transferHandler := delivery.NewTransferHandler(transferUC)
	walletHandler := delivery.NewWalletHandler(walletUC)
	webhookHandler := delivery.NewWebhookHandler(webhookUC)
	cryptoHandler := delivery.NewCryptoHandler(cryptoUC)
	exchangeHandler := delivery.NewExchangeHandler(exchangeUC)
	fiatHandler := delivery.NewFiatHandler(fiatUC)
	notifHandler := delivery.NewNotificationHandler(notifUC)

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

	r.Use(middleware.CORSMiddleware())
	r.OPTIONS("/*any", middleware.CORSMiddleware())

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
			auth.POST("/2fa/login", authHandler.Login2FA)
		}

		api.POST("/webhooks/midtrans", webhookHandler.HandleMidtrans)
		api.POST("/webhooks/iris", webhookHandler.HandleIris)

		limiter := middleware.IPBasedRateLimiter(10, 1, 2*time.Second)

		api.Use(middleware.JWTAuth(jwtSecret))
		{
			api.GET("/auth/me", authHandler.GetMe)
			api.POST("/transfer", limiter, middleware.Require2FAIfEnabled(authUC), transferHandler.Transfer)
			api.POST("/topup", walletHandler.TopUp)
			api.POST("/topup/status", walletHandler.CheckTopUpStatus)
			api.POST("/topup/simulate-settlement", walletHandler.SimulateTopUpSettlement)
			api.GET("/transactions", walletHandler.GetTransactionHistory)
			api.GET("/wallet/dashboard", walletHandler.GetDashboard)

			api.POST("/auth/2fa/enable", authHandler.Enable2FA)
			api.POST("/auth/2fa/verify", authHandler.Verify2FA)
			api.POST("/auth/2fa/disable", authHandler.Disable2FA)
			api.GET("/auth/2fa/recovery-codes", authHandler.GetRecoveryCodes)
			api.POST("/auth/2fa/recovery-codes/regenerate", authHandler.RegenerateRecoveryCodes)
			api.POST("/auth/2fa/email-otp/send", authHandler.Send2FAEmailOTP)

			api.POST("/auth/password/email-otp/send", authHandler.SendChangePasswordEmailOTP)
			api.POST("/auth/password/change", authHandler.ChangePassword)

			api.POST("/auth/pin/setup", authHandler.SetupPIN)
			api.POST("/auth/pin/verify", authHandler.VerifyPIN)

			api.GET("/auth/biometric/challenge", authHandler.GetBiometricChallenge)
			api.POST("/auth/biometric/register", authHandler.RegisterBiometric)
			api.POST("/auth/biometric/verify", authHandler.VerifyBiometric)
			api.DELETE("/auth/biometric", authHandler.DisableBiometric)

			api.GET("/crypto/address", cryptoHandler.GetDepositAddress)
			api.POST("/crypto/withdraw", limiter, middleware.Require2FAIfEnabled(authUC), cryptoHandler.WithdrawCrypto)
			api.POST("/crypto/simulate-deposit", cryptoHandler.SimulateDeposit)

			api.GET("/exchange/rate", exchangeHandler.GetRate)
			api.POST("/exchange/swap", limiter, exchangeHandler.Swap)

			api.POST("/fiat/withdraw", limiter, middleware.Require2FAIfEnabled(authUC), fiatHandler.WithdrawFiat)

			// Notifications
			api.GET("/notifications", notifHandler.GetNotifications)
			api.GET("/notifications/unread-count", notifHandler.GetUnreadCount)
			api.PATCH("/notifications/read-all", notifHandler.MarkAllAsRead)
			api.PATCH("/notifications/:id/read", notifHandler.MarkAsRead)
			api.DELETE("/notifications/:id", notifHandler.DeleteNotification)
		}
	}

	r.GET("/ping", middleware.JWTAuth(jwtSecret), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "pong",
		})
	})

	r.GET("/health", func(c *gin.Context) {
		sqlDB, err := db.DB()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"status":  "error",
				"message": "database connection unavailable: " + err.Error(),
			})
			return
		}
		if err := sqlDB.Ping(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"status":  "error",
				"message": "database ping failed: " + err.Error(),
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"status":  "healthy",
			"message": "database connection verified",
		})
	})

	srv := &http.Server{
		Addr:    ":" + port,
		Handler: r,
	}

	go func() {
		slog.Info("server starting", "port", port)
		slog.Info("swagger docs", "url", "http://localhost:"+port+"/swagger/index.html")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server failed to start", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	slog.Info("shutting down server...")

	cancel()

	ctxShutdown, cancelShutdown := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelShutdown()
	if err := srv.Shutdown(ctxShutdown); err != nil {
		slog.Error("server forced to shutdown", "error", err)
		os.Exit(1)
	}

	slog.Info("server exiting gracefully")
}
