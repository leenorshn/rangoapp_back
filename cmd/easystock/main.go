package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"rangoapp/internal/easystock/api"
	"rangoapp/internal/easystock/authn"
	"rangoapp/internal/easystock/billing"
	"rangoapp/internal/easystock/commerce"
	"rangoapp/internal/easystock/grpcserver"
	"rangoapp/internal/easystock/identity"
	"rangoapp/internal/easystock/legacyimport"
	"rangoapp/internal/easystock/media"
	"rangoapp/internal/easystock/migrate"
	"rangoapp/internal/easystock/notify"
	"rangoapp/internal/easystock/readmodel"
	"rangoapp/internal/easystock/reminders"
	"rangoapp/internal/easystock/reports"
	"rangoapp/internal/easystock/subscription"

	commercev1 "rangoapp/internal/easystock/gen/commercev1"
	subscriptionv1 "rangoapp/internal/easystock/gen/subscriptionv1"

	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatal("usage: easystock [migrate|grpc|api|worker|import]")
	}
	ctx := context.Background()
	switch os.Args[1] {
	case "migrate":
		pool := mustPool(ctx, "DATABASE_URL")
		defer pool.Close()
		if err := migrate.Up(ctx, pool); err != nil {
			log.Fatal(err)
		}
	case "grpc":
		serveGRPC(ctx)
	case "api":
		serveAPI(ctx)
	case "worker":
		serveWorker(ctx)
	case "import":
		runImport(ctx)
	default:
		log.Fatal("unknown command")
	}
}

func serveGRPC(ctx context.Context) {
	addr := env("GRPC_LISTEN", "127.0.0.1:7443")
	if os.Getenv("GRPC_INSECURE_DEV") != "true" && os.Getenv("GRPC_TLS_CERT") == "" && !isLoopback(addr) {
		log.Fatal("gRPC refuses a non-loopback listen without TLS; set GRPC_TLS_CERT or GRPC_INSECURE_DEV for a private network")
	}
	pool := mustPool(ctx, "DATABASE_URL_APP")
	defer pool.Close()
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatal(err)
	}
	server := grpc.NewServer(
		grpc.MaxRecvMsgSize(1<<20),
		grpc.MaxSendMsgSize(1<<20),
		grpc.ChainUnaryInterceptor(
			grpcserver.RecoveryInterceptor,
			grpcserver.LoggingInterceptor,
			grpcserver.AuthInterceptor(mustSecret("INTERNAL_CONTEXT_SECRET")),
		),
	)
	commercev1.RegisterCommerceServiceServer(server, grpcserver.NewCommerce(commerce.New(pool)))
	subscriptionv1.RegisterSubscriptionServiceServer(server, grpcserver.NewSubscription(subscription.New(pool, nil)))
	healthServer := health.NewServer()
	healthpb.RegisterHealthServer(server, healthServer)
	healthServer.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)
	go func() {
		log.Printf("easystock grpc listening on %s", addr)
		if err := server.Serve(lis); err != nil {
			log.Fatal(err)
		}
	}()
	waitForSignal()
	stopped := make(chan struct{})
	go func() {
		server.GracefulStop()
		close(stopped)
	}()
	select {
	case <-stopped:
	case <-time.After(20 * time.Second):
		server.Stop()
	}
}

func serveAPI(ctx context.Context) {
	pool := mustPool(ctx, "DATABASE_URL_IDENTITY")
	defer pool.Close()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	handler := graphQLHandler(ctx, pool)
	mux.Handle("POST /graphql", handler)
	mux.Handle("OPTIONS /graphql", handler)
	if os.Getenv("GRAPHQL_PLAYGROUND") == "true" {
		mux.Handle("GET /playground", api.Playground("EasyStock", "/graphql"))
	}
	server := &http.Server{Addr: ":" + env("PORT", "8080"), Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	go func() {
		log.Printf("easystock api listening on %s", server.Addr)
		if os.Getenv("GRAPHQL_PLAYGROUND") == "true" {
			log.Printf("GraphQL playground: http://localhost:%s/playground", env("PORT", "8080"))
		}
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()
	waitForSignal()
	shutdown, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	_ = server.Shutdown(shutdown)
}

func graphQLHandler(ctx context.Context, identityPool *pgxpool.Pool) http.Handler {
	members := identity.New(identityPool)
	cfg, err := loadAuthConfig()
	if err != nil {
		log.Fatal(err)
	}
	production := os.Getenv("APP_ENV") == "production"
	sms, err := openSMS(production)
	if err != nil {
		log.Fatal(err)
	}
	limiter := authn.Limiter(authn.NewPGLimiter(identityPool))
	if rawURL := os.Getenv("REDIS_URL"); rawURL != "" {
		redisLimiter, err := authn.NewRedisLimiter(rawURL)
		if err != nil {
			log.Fatal(err)
		}
		limiter = redisLimiter
	} else if production {
		log.Printf("REDIS_URL est vide: les limites de débit utilisent PostgreSQL")
	}
	authSvc := authn.New(identityPool, cfg, sms, limiter)
	store, err := mediaStore()
	if err != nil {
		log.Fatal(err)
	}
	deps := api.Deps{
		ContextSecret: os.Getenv("INTERNAL_CONTEXT_SECRET"),
		Identity:      members,
		Auth:          authSvc,
		Billing:       billing.New(identityPool, store),
	}
	if os.Getenv("DATABASE_URL_APP") != "" {
		appPool := mustPool(ctx, "DATABASE_URL_APP")
		deps.Commerce = commerce.New(appPool)
		if store != nil {
			deps.Commerce.SetObjects(store)
		}
		deps.Reads = readmodel.New(appPool)
		deps.Reports = reports.New(appPool)
	}
	if addr := os.Getenv("COMMERCE_GRPC_ADDR"); addr != "" {
		conn := mustCommerceConn(addr)
		deps.Sales = commercev1.NewCommerceServiceClient(conn)
		deps.Plans = subscriptionv1.NewSubscriptionServiceClient(conn)
	}
	schema, err := api.Schema(deps)
	if err != nil {
		log.Fatal(err)
	}
	session := api.SessionConfig{
		Auth:               authSvc,
		AllowIntrospection: os.Getenv("GRAPHQL_PLAYGROUND") == "true",
		CORSOrigins:        splitCSV(os.Getenv("AUTH_CORS_ORIGINS")),
		CookieSecure:       production || os.Getenv("AUTH_COOKIE_SECURE") == "true",
	}
	return api.NewHTTP(schema, session)
}

func serveWorker(ctx context.Context) {
	production := os.Getenv("APP_ENV") == "production"
	sms, err := openSMS(production)
	if err != nil {
		log.Fatal(err)
	}
	push, err := notify.OpenPush(env("PUSH_PROVIDER", "dev"), os.Getenv("PUSH_HTTP_URL"), os.Getenv("PUSH_HTTP_TOKEN"), production)
	if err != nil {
		log.Fatal(err)
	}
	pool := mustPool(ctx, "DATABASE_URL_JOB")
	defer pool.Close()
	worker := reminders.New(pool, sms, push)
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() {
		waitForSignal()
		cancel()
	}()
	log.Printf("easystock worker ticking every 5 minutes")
	worker.Run(runCtx)
}

func openSMS(production bool) (notify.SMSProvider, error) {
	return notify.OpenSMS(notify.SMSOptions{
		Provider:   env("SMS_PROVIDER", "dev"),
		HTTPURL:    os.Getenv("SMS_HTTP_URL"),
		HTTPToken:  os.Getenv("SMS_HTTP_TOKEN"),
		UzaBaseURL: os.Getenv("SMS_UZA_BASE_URL"),
		UzaToken:   os.Getenv("SMS_UZA_TOKEN"),
		UzaSender:  os.Getenv("SMS_UZA_SENDER"),
		Production: production,
	})
}

func loadAuthConfig() (authn.Config, error) {
	cfg := authn.DefaultConfig()
	cfg.JWTSecret = os.Getenv("AUTH_JWT_SECRET")
	cfg.OTPSecret = os.Getenv("AUTH_OTP_SECRET")
	if region := os.Getenv("PHONE_DEFAULT_REGION"); region != "" {
		cfg.Region = region
	}
	if len(cfg.JWTSecret) < 32 || len(cfg.OTPSecret) < 32 {
		return cfg, errors.New("AUTH_JWT_SECRET and AUTH_OTP_SECRET must be at least 32 characters")
	}
	cfg.SeededOTP = authn.DevelopmentOTPSeed()
	return cfg, nil
}

func splitCSV(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func mediaStore() (media.Store, error) {
	bucket := os.Getenv("S3_BUCKET")
	if bucket == "" {
		return nil, nil
	}
	pathStyle := os.Getenv("S3_USE_PATH_STYLE") == "true"
	return media.OpenS3(media.Config{
		Endpoint:  os.Getenv("S3_ENDPOINT"),
		Region:    env("S3_REGION", "us-east-1"),
		Bucket:    bucket,
		AccessKey: os.Getenv("S3_ACCESS_KEY_ID"),
		SecretKey: os.Getenv("S3_SECRET_ACCESS_KEY"),
		PathStyle: pathStyle,
	})
}

func mustCommerceConn(addr string) *grpc.ClientConn {
	var creds credentials.TransportCredentials
	if ca := os.Getenv("GRPC_TLS_CA"); ca != "" {
		loaded, err := credentials.NewClientTLSFromFile(ca, "")
		if err != nil {
			log.Fatal(err)
		}
		creds = loaded
	} else if os.Getenv("GRPC_INSECURE_DEV") == "true" || isLoopback(addr) {
		creds = insecure.NewCredentials()
	} else {
		log.Fatal("COMMERCE_GRPC_ADDR requires GRPC_TLS_CA, or GRPC_INSECURE_DEV on a private network")
	}
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(creds))
	if err != nil {
		log.Fatal(err)
	}
	return conn
}

func runImport(ctx context.Context) {
	dry := false
	path := ""
	for _, arg := range os.Args[2:] {
		if arg == "--dry-run" {
			dry = true
			continue
		}
		path = arg
	}
	if path == "" {
		log.Fatal("usage: easystock import [--dry-run] bundle.json")
	}
	body, err := os.ReadFile(path)
	if err != nil {
		log.Fatal(err)
	}
	var bundle legacyimport.Bundle
	if err := json.Unmarshal(body, &bundle); err != nil {
		log.Fatal(err)
	}
	pool := mustPool(ctx, "DATABASE_URL")
	defer pool.Close()
	report, err := legacyimport.Apply(ctx, pool, bundle, dry)
	if err != nil {
		log.Fatal(err)
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(report); err != nil {
		log.Fatal(err)
	}
	for _, total := range report.Totals {
		if !total.Match {
			os.Exit(2)
		}
	}
}

func mustPool(ctx context.Context, key string) *pgxpool.Pool {
	pool, err := pgxpool.New(ctx, os.Getenv(key))
	if err != nil {
		log.Fatal(err)
	}
	return pool
}

func mustSecret(key string) string {
	value := os.Getenv(key)
	if len(value) < 32 {
		log.Fatalf("%s must be at least 32 characters", key)
	}
	return value
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func isLoopback(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return false
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func waitForSignal() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	<-ch
}
