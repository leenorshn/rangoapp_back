package database

import (
	"context"
	"log"
	"os"
	"strconv"
	"sync"
	"time"

	"rangoapp/utils"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var (
	dbInstance *DB
	once       sync.Once // Protects dbInstance initialization from race conditions
	// memoryCache *MemoryCache // Disabled - Redis not configured
)

// Default timeout values
const (
	defaultDBTimeout      = 5 * time.Second  // Default timeout for DB operations
	defaultConnectTimeout = 10 * time.Second // Default timeout for connection
)

// getDBTimeout returns the configured database operation timeout
// Can be configured via DB_TIMEOUT_SECONDS environment variable
func getDBTimeout() time.Duration {
	timeoutStr := os.Getenv("DB_TIMEOUT_SECONDS")
	if timeoutStr == "" {
		return defaultDBTimeout
	}

	timeoutSeconds, err := strconv.Atoi(timeoutStr)
	if err != nil || timeoutSeconds <= 0 {
		utils.Warning("Invalid DB_TIMEOUT_SECONDS value '%s', using default: %v", timeoutStr, defaultDBTimeout)
		return defaultDBTimeout
	}

	timeout := time.Duration(timeoutSeconds) * time.Second
	if timeout < 1*time.Second {
		utils.Warning("DB_TIMEOUT_SECONDS too small (%d), using minimum: 1s", timeoutSeconds)
		return 1 * time.Second
	}
	if timeout > 60*time.Second {
		utils.Warning("DB_TIMEOUT_SECONDS too large (%d), using maximum: 60s", timeoutSeconds)
		return 60 * time.Second
	}

	return timeout
}

// getConnectTimeout returns the configured connection timeout
// Can be configured via DB_CONNECT_TIMEOUT_SECONDS environment variable
func getConnectTimeout() time.Duration {
	timeoutStr := os.Getenv("DB_CONNECT_TIMEOUT_SECONDS")
	if timeoutStr == "" {
		return defaultConnectTimeout
	}

	timeoutSeconds, err := strconv.Atoi(timeoutStr)
	if err != nil || timeoutSeconds <= 0 {
		utils.Warning("Invalid DB_CONNECT_TIMEOUT_SECONDS value '%s', using default: %v", timeoutStr, defaultConnectTimeout)
		return defaultConnectTimeout
	}

	timeout := time.Duration(timeoutSeconds) * time.Second
	if timeout < 1*time.Second {
		utils.Warning("DB_CONNECT_TIMEOUT_SECONDS too small (%d), using minimum: 1s", timeoutSeconds)
		return 1 * time.Second
	}
	if timeout > 120*time.Second {
		utils.Warning("DB_CONNECT_TIMEOUT_SECONDS too large (%d), using maximum: 120s", timeoutSeconds)
		return 120 * time.Second
	}

	return timeout
}

// GetDBContext returns a context with the configured database timeout
// This should be used for all database operations
func GetDBContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), getDBTimeout())
}

// getMaxRetries returns the configured maximum number of retry attempts
func getMaxRetries() int {
	retriesStr := os.Getenv("DB_MAX_RETRIES")
	if retriesStr == "" {
		return 3 // Default: 3 retries
	}

	retries, err := strconv.Atoi(retriesStr)
	if err != nil || retries <= 0 {
		utils.Warning("Invalid DB_MAX_RETRIES value '%s', using default: 3", retriesStr)
		return 3
	}

	if retries > 10 {
		utils.Warning("DB_MAX_RETRIES too large (%d), using maximum: 10", retries)
		return 10
	}

	return retries
}

// connectWithRetry attempts to connect with exponential backoff retry
func connectWithRetry(client *mongo.Client, ctx context.Context, maxRetries int, initialDelay time.Duration) error {
	var lastErr error
	delay := initialDelay

	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			utils.Info("Retrying MongoDB connection (attempt %d/%d) after %v...", attempt+1, maxRetries, delay)
			time.Sleep(delay)
			delay *= 2 // Exponential backoff
		}

		err := client.Connect(ctx)
		if err == nil {
			// Ping to verify connection
			err = client.Ping(ctx, nil)
			if err == nil {
				return nil
			}
		}

		lastErr = err
		utils.LogError(err, "MongoDB connection attempt failed")
	}

	return utils.WrapError(lastErr, "Failed to connect to MongoDB after retries")
}

type DB struct {
	client   *mongo.Client
	database *mongo.Database
	// cacheHelper *CacheHelper // Cache helper (disabled - Redis not configured)
}

func (db *DB) Client() *mongo.Client {
	return db.client
}

func ConnectDB() *DB {
	// Use sync.Once to ensure thread-safe initialization
	once.Do(func() {
		dbUrl := os.Getenv("MONGO_URI")
		if dbUrl == "" {
			log.Fatal("MONGO_URI environment variable is required")
		}

		dbName := os.Getenv("MONGO_DB_NAME")
		if dbName == "" {
			dbName = "rangodb"
		}

		connectTimeout := getConnectTimeout()
		utils.Info("Connecting to MongoDB with timeout: %v", connectTimeout)

		// Configure MongoDB client with optimized connection pool settings for Cloud Run
		clientOptions := options.Client().ApplyURI(dbUrl).
			SetMaxPoolSize(50).                         // Maximum number of connections in the pool
			SetMinPoolSize(5).                          // Minimum number of connections to maintain
			SetMaxConnIdleTime(30 * time.Second).       // Close connections after 30s of inactivity
			SetConnectTimeout(connectTimeout).          // Timeout for initial connection
			SetServerSelectionTimeout(5 * time.Second). // Timeout for server selection
			SetSocketTimeout(30 * time.Second).         // Timeout for socket operations
			SetHeartbeatInterval(10 * time.Second)      // Heartbeat interval for connection monitoring

		client, err := mongo.NewClient(clientOptions)
		if err != nil {
			log.Fatal("Error creating MongoDB client:", err)
		}

		ctx, cancel := context.WithTimeout(context.Background(), connectTimeout)
		defer cancel()

		// Connect with retry logic
		maxRetries := getMaxRetries()
		initialDelay := 2 * time.Second
		err = connectWithRetry(client, ctx, maxRetries, initialDelay)
		if err != nil {
			utils.Error("Failed to connect to MongoDB after %d retries: %v", maxRetries, err)
			log.Fatal("Error connecting to MongoDB:", err)
		}

		// Ping the primary to verify connection with timeout
		pingCtx, pingCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer pingCancel()
		if err := client.Ping(pingCtx, nil); err != nil {
			utils.Error("Failed to ping MongoDB: %v", err)
			log.Fatal("Error pinging MongoDB:", err)
		}

		utils.Info("Connected to MongoDB successfully (maxPoolSize: 50, minPoolSize: 5)")

		// Cache disabled - Redis not configured
		// To enable cache, uncomment and configure Redis:
		// if memoryCache == nil {
		//     memoryCache = NewMemoryCache()
		// }
		// cacheHelper := NewCacheHelper(memoryCache)

		dbInstance = &DB{
			client:   client,
			database: client.Database(dbName),
			// cacheHelper: cacheHelper, // Disabled
		}

		// Create indexes
		createIndexes(dbInstance)
	})

	return dbInstance
}

func colHelper(db *DB, collectionName string) *mongo.Collection {
	return db.database.Collection(collectionName)
}

func createIndexes(db *DB) {
	ctx := context.Background()

	// Users indexes
	userCollection := colHelper(db, "users")
	userIndexes := []mongo.IndexModel{
		{
			Keys:    map[string]interface{}{"uid": 1},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys: map[string]interface{}{"companyId": 1},
		},
		{
			Keys: map[string]interface{}{"storeIds": 1},
		},
	}
	_, err := userCollection.Indexes().CreateMany(ctx, userIndexes)
	if err != nil {
		utils.LogError(err, "Failed to create user indexes")
	}

	// Stores indexes
	storeCollection := colHelper(db, "stores")
	storeIndexes := []mongo.IndexModel{
		{
			Keys: map[string]interface{}{"companyId": 1},
		},
	}
	_, err = storeCollection.Indexes().CreateMany(ctx, storeIndexes)
	if err != nil {
		utils.LogError(err, "Failed to create store indexes")
	}

	// Products indexes
	productCollection := colHelper(db, "products")
	productIndexes := []mongo.IndexModel{
		{
			Keys: map[string]interface{}{"storeId": 1},
		},
		{
			// Compound index for storeId + name (for search queries)
			Keys: bson.M{
				"storeId": 1,
				"name":    1,
			},
		},
	}
	_, err = productCollection.Indexes().CreateMany(ctx, productIndexes)
	if err != nil {
		utils.LogError(err, "Failed to create product indexes")
	}

	// Clients indexes
	clientCollection := colHelper(db, "clients")
	clientIndexes := []mongo.IndexModel{
		{
			Keys: map[string]interface{}{"storeId": 1},
		},
	}
	_, err = clientCollection.Indexes().CreateMany(ctx, clientIndexes)
	if err != nil {
		utils.LogError(err, "Failed to create client indexes")
	}

	// Providers indexes
	providerCollection := colHelper(db, "providers")
	providerIndexes := []mongo.IndexModel{
		{
			Keys: map[string]interface{}{"storeId": 1},
		},
	}
	_, err = providerCollection.Indexes().CreateMany(ctx, providerIndexes)
	if err != nil {
		utils.LogError(err, "Failed to create provider indexes")
	}

	// Factures indexes
	factureCollection := colHelper(db, "factures")
	factureIndexes := []mongo.IndexModel{
		{
			// Compound unique index on storeId and factureNumber
			Keys: bson.M{
				"storeId":       1,
				"factureNumber": 1,
			},
			Options: options.Index().SetUnique(true),
		},
		{
			// Simple index on storeId
			Keys: bson.M{"storeId": 1},
		},
		{
			// Compound index for storeId + createdAt (for period filters)
			Keys: bson.M{
				"storeId":   1,
				"createdAt": -1,
			},
		},
	}
	_, err = factureCollection.Indexes().CreateMany(ctx, factureIndexes)
	if err != nil {
		utils.LogError(err, "Failed to create facture indexes")
	}

	// RapportStore indexes
	rapportCollection := colHelper(db, "rapportStore")
	rapportIndexes := []mongo.IndexModel{
		{
			Keys: map[string]interface{}{"storeId": 1},
		},
		{
			Keys: map[string]interface{}{"productId": 1},
		},
	}
	_, err = rapportCollection.Indexes().CreateMany(ctx, rapportIndexes)
	if err != nil {
		utils.LogError(err, "Failed to create rapportStore indexes")
	}

	// Caisse transactions indexes
	transCollection := colHelper(db, "trans")
	transIndexes := []mongo.IndexModel{
		{
			Keys: map[string]interface{}{"storeId": 1},
		},
		{
			Keys: map[string]interface{}{"currency": 1},
		},
		{
			Keys: map[string]interface{}{"date": -1}, // Descending for recent first
		},
		{
			Keys: map[string]interface{}{"operation": 1},
		},
		{
			// Compound index for common queries: storeId + currency + date
			Keys: bson.M{
				"storeId":  1,
				"currency": 1,
				"date":     -1,
			},
		},
		{
			// Compound index for storeId + createdAt
			Keys: bson.M{
				"storeId":   1,
				"createdAt": -1,
			},
		},
	}
	_, err = transCollection.Indexes().CreateMany(ctx, transIndexes)
	if err != nil {
		utils.LogError(err, "Failed to create trans indexes")
	}

	// Sales indexes for optimized queries
	saleCollection := colHelper(db, "sales")
	saleIndexes := []mongo.IndexModel{
		{
			Keys: map[string]interface{}{"storeId": 1},
		},
		{
			Keys: map[string]interface{}{"date": -1}, // Descending for recent first
		},
		{
			Keys: map[string]interface{}{"createdAt": -1}, // Descending for recent first
		},
		{
			Keys: map[string]interface{}{"currency": 1},
		},
		{
			// Compound index for common queries: storeId + createdAt (for period filters)
			Keys: bson.M{
				"storeId":   1,
				"createdAt": -1,
			},
		},
		{
			// Compound index for storeId + currency + createdAt
			Keys: bson.M{
				"storeId":   1,
				"currency":  1,
				"createdAt": -1,
			},
		},
		{
			// Compound index for storeId + date (for date-based queries)
			Keys: bson.M{
				"storeId": 1,
				"date":    -1,
			},
		},
		{
			// Index for clientId queries
			Keys: map[string]interface{}{"clientId": 1},
		},
	}
	_, err = saleCollection.Indexes().CreateMany(ctx, saleIndexes)
	if err != nil {
		utils.LogError(err, "Failed to create sales indexes")
	}

	// Subscriptions indexes
	subscriptionCollection := colHelper(db, "subscriptions")
	subscriptionIndexes := []mongo.IndexModel{
		{
			Keys:    map[string]interface{}{"companyId": 1},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys: map[string]interface{}{"status": 1},
		},
		{
			Keys: map[string]interface{}{"trialEndDate": 1},
		},
		{
			Keys: map[string]interface{}{"subscriptionEndDate": 1},
		},
	}
	_, err = subscriptionCollection.Indexes().CreateMany(ctx, subscriptionIndexes)
	if err != nil {
		utils.LogError(err, "Failed to create subscription indexes")
	}

	// Debts indexes
	debtCollection := colHelper(db, "debts")
	debtIndexes := []mongo.IndexModel{
		{
			Keys: map[string]interface{}{"storeId": 1},
		},
		{
			Keys: map[string]interface{}{"status": 1},
		},
		{
			Keys: map[string]interface{}{"createdAt": -1},
		},
		{
			// Compound index for storeId + status + createdAt
			Keys: bson.M{
				"storeId":   1,
				"status":    1,
				"createdAt": -1,
			},
		},
		{
			// Compound index for clientId + storeId
			Keys: bson.M{
				"clientId": 1,
				"storeId":  1,
			},
		},
	}
	_, err = debtCollection.Indexes().CreateMany(ctx, debtIndexes)
	if err != nil {
		utils.LogError(err, "Failed to create debt indexes")
	}

	// Inventories indexes
	inventoryCollection := colHelper(db, "inventories")
	inventoryIndexes := []mongo.IndexModel{
		{
			Keys: map[string]interface{}{"storeId": 1},
		},
		{
			Keys: map[string]interface{}{"status": 1},
		},
		{
			Keys: map[string]interface{}{"createdAt": -1},
		},
		{
			// Compound index for storeId + status + createdAt
			Keys: bson.M{
				"storeId":   1,
				"status":    1,
				"createdAt": -1,
			},
		},
	}
	_, err = inventoryCollection.Indexes().CreateMany(ctx, inventoryIndexes)
	if err != nil {
		utils.LogError(err, "Failed to create inventory indexes")
	}

	utils.Info("MongoDB indexes created successfully")
}
