package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"prakarsa-app/config"
	"prakarsa-app/infrastructure/datastore"
	"prakarsa-app/infrastructure/transport/clients"
	"prakarsa-app/infrastructure/transport/httpclient"
	"prakarsa-app/usecase"
	"prakarsa-app/utils"
	"prakarsa-app/utils/jwt"
	"time"

	httpDelivery "prakarsa-app/delivery/http"
	appMiddleware "prakarsa-app/delivery/middleware"
	pgsqlRepository "prakarsa-app/repository/pgsql"
	redisRepository "prakarsa-app/repository/redis"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	echoSwagger "github.com/swaggo/echo-swagger"
)

// @title Go Boilerplate
// @version 1.0.4
// @termsOfService http://swagger.io/terms/
// @securityDefinitions.apikey JwtToken
// @in header
// @name Authorization
func main() {
	// Load config
	configApp := config.LoadConfig()

	// Setup infra
	dbInstance, err := datastore.NewDatabase(configApp.DatabaseURL)
	utils.PanicIfNeeded(err)

	cacheInstance, err := datastore.NewCache(configApp.CacheURL)
	utils.PanicIfNeeded(err)

	hcNotif := httpclient.New(
		config.LoadConfig().BaseUrlAPI,
		httpclient.WithDefaultHeader("Accept", "application/json"),
	)

	// Setup repository
	redisRepo := redisRepository.NewRedisRepository(cacheInstance)
	collaborationRepo := pgsqlRepository.NewPgsqlCollaborationRepository(dbInstance)
	notif := clients.NewNotificationClient(hcNotif)

	// Setup Service
	jwtSvc := jwt.NewJWTService(configApp.JWTSecretKey)

	// Setup usecase
	ctxTimeout := time.Duration(configApp.ContextTimeout) * time.Second
	collaborationUC := usecase.NewCollaborationUsecase(collaborationRepo, redisRepo, ctxTimeout, notif)

	// Setup app middleware
	appMiddleware := appMiddleware.NewMiddleware(jwtSvc)

	// Setup route engine & middleware
	e := echo.New()
	e.Use(middleware.CORS())
	//e.Use(appMiddleware.Logger(nil))
	e.Use(appMiddleware.CustomLogger())
	e.Logger.Info("🚀 Server is alive and running")

	// Setup handler
	e.GET("/swagger/*", echoSwagger.WrapHandler)
	e.GET("/", func(c echo.Context) error {
		return c.NoContent(http.StatusOK)
	})

	httpDelivery.NewCollaborationHandler(e, appMiddleware, collaborationUC)

	// Start server
	go func() {
		if err := e.Start(":8080"); err != nil && err != http.ErrServerClosed {
			e.Logger.Fatal("shutting down the server")
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server with a timeout of 10 seconds.
	// Use a buffered channel to avoid missing signals as recommended for signal.Notify
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)
	<-quit
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(configApp.ContextTimeout)*time.Second)
	defer cancel()
	if err := e.Shutdown(ctx); err != nil {
		e.Logger.Fatal(err)
	}
}
