package app

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"go-service-template/internal/errorz"
	"go-service-template/internal/jwt"
	"go-service-template/internal/metrics"
	"go-service-template/internal/models"
	"go-service-template/internal/repository"
	"go-service-template/internal/transport/http/server"
	v1 "go-service-template/internal/transport/http/v1"
	"go-service-template/internal/ws"
	swaggerui "go-service-template/pkg/swagger-ui"

	"go-service-template/internal/config"
	"go-service-template/internal/db"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"golang.org/x/time/rate"

	balancerepo "go-service-template/internal/repository/balance"
	blockrepo "go-service-template/internal/repository/block"
	dailyrewardrepo "go-service-template/internal/repository/daily_reward"
	announcementrepo "go-service-template/internal/repository/announcement"
	hugrepo "go-service-template/internal/repository/hug"
	intimacyrepo "go-service-template/internal/repository/intimacy"
	tokenrepo "go-service-template/internal/repository/token"
	userrepo "go-service-template/internal/repository/user"

	hugservice "go-service-template/internal/service/hug"
	userservice "go-service-template/internal/service/user"
	"go-service-template/internal/telegram"

	adminhandler "go-service-template/internal/transport/http/v1/admin"
	hughandler "go-service-template/internal/transport/http/v1/hug"
	userhandler "go-service-template/internal/transport/http/v1/user"

	custommiddleware "go-service-template/internal/transport/http/middleware"
)

type App struct {
	cfg        *config.Config
	l          *slog.Logger
	e          *echo.Echo
	metricsSrv *http.Server
	dbPool     *pgxpool.Pool
	hub        *ws.Hub
	stopJobs   context.CancelFunc
}

// New creates and initializes a new instance of App
func New(ctx context.Context, cfg *config.Config, l *slog.Logger) (*App, error) {
	a := &App{
		cfg: cfg,
		l:   l,
	}

	if err := a.initDB(ctx); err != nil {
		return nil, err
	}

	if err := a.migrateDB(); err != nil {
		return nil, err
	}

	// Metrics server (separate port, exposes /metrics for Prometheus scraping)
	a.metricsSrv = metrics.Register(a.cfg.MetricsSrv.Addr, a.dbPool)

	jwtManager := jwt.NewManager(
		a.cfg.JWT.Secret,
		time.Duration(a.cfg.JWT.AccessTokenDurationSec)*time.Second,
		time.Duration(a.cfg.JWT.RefreshTokenDurationSec)*time.Second,
	)

	// Repositories
	userRepo := userrepo.New(a.dbPool)
	hugRepo := hugrepo.New(a.dbPool)
	balanceRepo := balancerepo.New(a.dbPool)
	dailyRewardRepo := dailyrewardrepo.New(a.dbPool)
	blockRepoInst := blockrepo.New(a.dbPool)
	refreshTokenRepo := tokenrepo.New(a.dbPool)
	intimacyRepoInst := intimacyrepo.New(a.dbPool)
	announcementRepo := announcementrepo.New(a.dbPool)

	// Transactor for database transactions
	transactor := repository.NewTransactor(a.dbPool)

	// Services
	userService := userservice.New(
		userRepo,
		jwtManager,
		userservice.WithBalanceRepo(balanceRepo),
		userservice.WithRefreshTokenRepo(refreshTokenRepo),
		userservice.WithTransactor(transactor),
		userservice.WithAnnouncementRepo(announcementRepo),
	)
	hugService := hugservice.New(hugRepo, balanceRepo, dailyRewardRepo, userRepo, blockRepoInst, intimacyRepoInst, jwtManager, transactor)

	// Telegram: client, bot, notifier, link store & login store
	tgClient := telegram.New(a.cfg.Telegram.BotToken)
	tgLinkStore := telegram.NewLinkStore()
	tgLoginStore := telegram.NewLoginStore()
	tgBot := telegram.NewBot(tgClient, tgLinkStore, userRepo, hugService, a.l)
	tgNotifier := telegram.NewNotifier(tgClient, tgBot, userRepo, a.l)

	// Telegram link store for user service (generating deep-link tokens)
	userService.SetTelegramLinkStore(tgLinkStore, a.cfg.Telegram.BotUsername)

	// Telegram login: wire login store + service into bot
	tgBot.SetLoginStore(tgLoginStore, userService)

	// WebSocket Hub
	a.hub = ws.NewHub(jwtManager)

	hugService.SetHugCompletedCallback(func(item *models.HugFeedItem, bonusCoins int32, comment *string) {
		a.hub.Broadcast("hug_completed", hughandler.ToFeedItemDTO(item))
		tgNotifier.NotifyHugCompleted(context.Background(), item.GiverID, item.ReceiverID, item.HugType, bonusCoins, comment)
	})
	hugService.SetHugSuggestionCallback(func(targetUserID uuid.UUID, item *models.PendingHugInboxItem, comment *string) {
		a.hub.SendToUser(targetUserID, "hug_suggestion", hughandler.ToPendingInboxItemDTO(item))
		tgNotifier.NotifyHugSuggestion(context.Background(), targetUserID, item.ID, item.GiverID, item.HugType, comment)
	})
	hugService.SetHugDeclinedCallback(func(targetUserID uuid.UUID, hugID uuid.UUID, receiverID uuid.UUID) {
		a.hub.SendToUser(targetUserID, "hug_declined", map[string]string{"hug_id": hugID.String(), "receiver_id": receiverID.String()})
		go tgNotifier.NotifyHugDeclined(context.Background(), targetUserID, receiverID)
	})
	hugService.SetHugCancelledCallback(func(targetUserID uuid.UUID, hugID uuid.UUID) {
		a.hub.SendToUser(targetUserID, "hug_cancelled", map[string]string{"hug_id": hugID.String()})
	})

	userService.SetAnnouncementCreatedCallback(func(ann *models.Announcement) {
		a.hub.Broadcast("announcement", map[string]any{
			"id":         ann.ID.String(),
			"message":    ann.Message,
			"created_at": ann.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		})
	})
	userService.SetAnnouncementRemovedCallback(func(id uuid.UUID) {
		a.hub.Broadcast("announcement_removed", map[string]string{"id": id.String()})
	})
	userService.SetPromotionUpdatedCallback(func() {
		a.hub.Broadcast("vips_updated", nil)
	})

	// Handlers
	userHandler := userhandler.New(userService, jwtManager, a.cfg.JWT.CookieSecure)
	userHandler.SetTelegramLoginStore(tgLoginStore, a.cfg.Telegram.BotUsername)
	hugHandler := hughandler.New(hugService, userService)
	adminHandler := adminhandler.New(userService)

	if err := a.initEcho(); err != nil {
		return nil, err
	}

	apiGroup := a.e.Group("/api/v1")

	strictMiddlewares := []v1.StrictMiddlewareFunc{
		custommiddleware.StrictErrorMiddleware,
	}

	strictServer := server.New(userHandler, hugHandler, adminHandler)
	strictHandler := v1.NewStrictHandler(strictServer, strictMiddlewares)

	oapiValidationMiddleware, err := custommiddleware.OpenAPIValidationMiddleware(jwtManager)
	if err != nil {
		return nil, fmt.Errorf("failed to create OpenAPI validation middleware: %w", err)
	}
	apiGroup.Use(oapiValidationMiddleware)

	v1.RegisterHandlers(apiGroup, strictHandler)

	// WebSocket endpoint (outside OpenAPI validation)
	a.e.GET("/api/v1/ws", a.hub.HandleWS)

	// Background jobs
	jobCtx, jobCancel := context.WithCancel(context.Background())
	a.stopJobs = jobCancel

	// Telegram bot (long-polling)
	go tgBot.Run(jobCtx)

	// Expire stale pending hugs every 5 minutes.
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-jobCtx.Done():
				return
			case <-ticker.C:
				if err := hugService.ExpirePendingHugs(jobCtx); err != nil {
					a.l.Error("failed to expire pending hugs", "error", err)
				}
			}
		}
	}()

	// Apply intimacy decay every hour.
	go func() {
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-jobCtx.Done():
				return
			case <-ticker.C:
				if err := hugService.ApplyIntimacyDecay(jobCtx); err != nil {
					a.l.Error("failed to apply intimacy decay", "error", err)
				}
			}
		}
	}()

	// Clear expired VIP promotions every minute.
	go func() {
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-jobCtx.Done():
				return
			case <-ticker.C:
				if _, err := userService.ClearExpiredPromotions(jobCtx); err != nil {
					a.l.Error("failed to clear expired promotions", "error", err)
				}
			}
		}
	}()

	return a, nil
}

// Start performs a start of all functional services
func (a *App) Start(errChan chan<- error) {
	a.l.Info("Starting...")

	// Start metrics server on a separate port
	metrics.StartServer(a.metricsSrv, a.l)

	if err := a.e.Start(a.cfg.HttpSrv.Addr); err != nil {
		errChan <- err
	}
}

// Stop performs a graceful shutdown for all components
func (a *App) Stop(ctx context.Context) error {
	a.l.Info("[!] Shutting down...")

	// Cancel background jobs first.
	if a.stopJobs != nil {
		a.stopJobs()
	}

	var stopErr error

	a.l.Info("Stopping http server...")
	if err := a.e.Shutdown(ctx); err != nil {
		stopErr = errors.Join(stopErr, fmt.Errorf("failed to shutdown http server: %w", err))
	}

	a.l.Info("Stopping metrics server...")
	if err := metrics.StopServer(ctx, a.metricsSrv); err != nil {
		stopErr = errors.Join(stopErr, fmt.Errorf("failed to shutdown metrics server: %w", err))
	}

	a.l.Info("Closing database pool...")
	a.dbPool.Close()

	if stopErr != nil {
		return stopErr
	}

	a.l.Info("Stopped gracefully")
	return nil
}

// initDB sets up PostgreSQL db with properly configured pool settings.
func (a *App) initDB(ctx context.Context) error {
	poolCfg, err := pgxpool.ParseConfig(a.cfg.Postgres.URL)
	if err != nil {
		return fmt.Errorf("failed to parse db config: %w", err)
	}

	poolCfg.MaxConns = a.cfg.Postgres.MaxConns
	poolCfg.MinConns = a.cfg.Postgres.MinConns
	poolCfg.MaxConnLifetime = time.Duration(a.cfg.Postgres.MaxConnLifetime) * time.Second
	poolCfg.MaxConnIdleTime = 5 * time.Minute

	dbPool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return fmt.Errorf("failed to init db connection: %w", err)
	}
	a.dbPool = dbPool
	return nil
}

// migrateDB performs a migration to ensure the schema is up to date
func (a *App) migrateDB() error {
	conn := sql.OpenDB(stdlib.GetConnector(*a.dbPool.Config().ConnConfig))
	defer conn.Close()

	return db.Migrate(conn)
}

// initEcho sets up a new Echo instance with logger and CORS
func (a *App) initEcho() error {
	a.e = echo.New()
	a.e.HideBanner = true
	a.e.HidePort = true
	a.e.Pre(middleware.RemoveTrailingSlashWithConfig(middleware.TrailingSlashConfig{
		Skipper: func(c echo.Context) bool {
			return strings.HasPrefix(c.Request().URL.Path, "/api/v1/swagger")
		},
	}))

	// CORS - allow all origins for now
	a.e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: a.cfg.CORS.AllowOrigins,
		AllowMethods: []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodOptions},
		AllowHeaders: []string{echo.HeaderOrigin, echo.HeaderContentType, echo.HeaderAccept, echo.HeaderAuthorization},
	}))

	a.e.Use(middleware.RequestLoggerWithConfig(middleware.RequestLoggerConfig{
		LogStatus:   true,
		LogURI:      true,
		LogError:    true,
		LogRemoteIP: true,
		HandleError: true,
		LogValuesFunc: func(c echo.Context, v middleware.RequestLoggerValues) error {
			if v.Error == nil {
				a.l.LogAttrs(context.Background(), slog.LevelInfo, "REQUEST",
					slog.String("uri", v.URI),
					slog.Int("status", v.Status),
					slog.String("ip", v.RemoteIP),
					slog.String("latency", time.Since(v.StartTime).String()),
				)
			} else {
				a.l.LogAttrs(context.Background(), slog.LevelError, "REQUEST_ERROR",
					slog.String("uri", v.URI),
					slog.Int("status", v.Status),
					slog.String("ip", v.RemoteIP),
					slog.String("latency", time.Since(v.StartTime).String()),
					slog.String("err", v.Error.Error()),
				)
			}
			return nil
		},
	}))
	a.e.Use(metrics.Middleware())
	a.e.Use(middleware.Recover())
	a.e.Use(custommiddleware.AuthRateLimitMiddleware(rate.Limit(2), 5, map[string]custommiddleware.PathRateLimit{
		// Registration: 2 accounts per hour per IP
		"/api/v1/auth/register": {
			Rate:  rate.Every(30 * time.Minute), // 1 token per 30 min = 2/hour sustained
			Burst: 2,                            // allow 2 immediate, then wait
			TTL:   1 * time.Hour,
		},
		// Username check: more lenient (fired on every keystroke with debounce)
		"/api/v1/auth/check-username": {
			Rate:  rate.Limit(5),
			Burst: 10,
			TTL:   5 * time.Minute,
		},
		// Telegram login init: 5 per minute per IP
		"/api/v1/auth/telegram/init": {
			Rate:  rate.Every(12 * time.Second),
			Burst: 5,
			TTL:   1 * time.Minute,
		},
		// Telegram login poll: more lenient (polled every 2 seconds)
		"/api/v1/auth/telegram/poll": {
			Rate:  rate.Limit(2),
			Burst: 5,
			TTL:   5 * time.Minute,
		},
	}))

	a.e.GET("/api/v1/openapi.json", func(c echo.Context) error {
		spec, err := v1.GetSwagger()
		if err != nil {
			slog.Error("failed to get swagger spec", "error", err)
			return echo.NewHTTPError(http.StatusInternalServerError, errorz.ErrInternalServerError.Error())
		}
		return c.JSON(http.StatusOK, spec)
	})

	swaggerUIHandler, err := swaggerui.Handler()
	if err != nil {
		return fmt.Errorf("failed to get swagger ui handler: %w", err)
	}

	uiHandler := http.StripPrefix("/api/v1/swagger", swaggerUIHandler)
	a.e.GET("/api/v1/swagger/*", echo.WrapHandler(uiHandler))
	a.e.GET("/api/v1/swagger", func(c echo.Context) error {
		return c.Redirect(http.StatusMovedPermanently, "/api/v1/swagger/")
	})

	return nil
}
