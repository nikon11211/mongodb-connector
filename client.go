package mongodb

import (
	"context"
	"fmt"
	"slices"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/mongo/readpref"
)

type Indexes interface {
	CreateMany(ctx context.Context, models []mongo.IndexModel, opts ...options.Lister[options.CreateIndexesOptions]) ([]string, error)
}

type Collection interface {
	Indexes() Indexes
}

type Database interface {
	Collection(name string, opts ...options.Lister[options.CollectionOptions]) Collection
	ListCollectionNames(ctx context.Context, filter any, opts ...options.Lister[options.ListCollectionsOptions]) ([]string, error)
	CreateCollection(ctx context.Context, name string, opts ...options.Lister[options.CreateCollectionOptions]) error
}

type MongoClient interface {
	Ping(ctx context.Context, rp *readpref.ReadPref) error
	Disconnect(ctx context.Context) error
	Database(name string, opts ...options.Lister[options.DatabaseOptions]) Database
}

type clientAdapter struct {
	c *mongo.Client
}

func (a *clientAdapter) Ping(ctx context.Context, rp *readpref.ReadPref) error {
	return a.c.Ping(ctx, rp)
}

func (a *clientAdapter) Disconnect(ctx context.Context) error {
	return a.c.Disconnect(ctx)
}

func (a *clientAdapter) Database(name string, opts ...options.Lister[options.DatabaseOptions]) Database {
	return &databaseAdapter{db: a.c.Database(name, opts...)}
}

type databaseAdapter struct {
	db *mongo.Database
}

func (a *databaseAdapter) Collection(name string, opts ...options.Lister[options.CollectionOptions]) Collection {
	return &collectionAdapter{coll: a.db.Collection(name, opts...)}
}

func (a *databaseAdapter) ListCollectionNames(ctx context.Context, filter any, opts ...options.Lister[options.ListCollectionsOptions]) ([]string, error) {
	return a.db.ListCollectionNames(ctx, filter, opts...)
}

func (a *databaseAdapter) CreateCollection(ctx context.Context, name string, opts ...options.Lister[options.CreateCollectionOptions]) error {
	return a.db.CreateCollection(ctx, name, opts...)
}

type collectionAdapter struct {
	coll *mongo.Collection
}

func (a *collectionAdapter) Indexes() Indexes {
	return a.coll.Indexes()
}

type Client struct {
	Database
	client MongoClient
	ddl    Database
	config *Config
	logger Logger
}

func New(ctx context.Context, cfg *Config, logger Logger, opts ...ClientOption) (*Client, error) {
	const op = "mongodb.New"
	if cfg == nil {
		return nil, fmt.Errorf("%s: config cannot be nil", op)
	}

	o := defaultClientOptions()
	for _, opt := range opts {
		opt(o)
	}
	connectFn := o.connect
	if connectFn == nil {
		connectFn = connect
	}

	ddl, ddlClient, err := newDDLManager(ctx, cfg, connectFn, o.pingTimeout)
	if err != nil {
		logger.Error(fmt.Sprintf("%s: failed to connect with DDL credentials: %s", op, err))
		return nil, err
	}

	client, err := newDMLClient(ctx, cfg, logger, connectFn, o.pingTimeout)
	if err != nil {
		logger.Error(fmt.Sprintf("%s: failed to connect with DML credentials: %s", op, err))
		_ = ddlClient.Disconnect(context.Background())
		return nil, err
	}
	client.ddl = ddl
	return client, nil
}

func newDDLManager(ctx context.Context, cfg *Config, connectFn ConnectFunc, pingTimeout time.Duration) (Database, MongoClient, error) {
	const op = "mongodb.newDDLManager"
	clientOpts, err := buildClientOptions(cfg, cfg.DDLUser, cfg.DDLPassword, cfg.DDLAuthSource)
	if err != nil {
		return nil, nil, fmt.Errorf("%s: %w", op, err)
	}

	mongoClient, err := connectFn(clientOpts)
	if err != nil {
		return nil, nil, fmt.Errorf("%s: failed to connect: %w", op, err)
	}

	if err := ping(ctx, mongoClient, pingTimeout); err != nil {
		_ = mongoClient.Disconnect(context.Background())
		return nil, nil, fmt.Errorf("%s: ping failed: %w", op, err)
	}

	return mongoClient.Database(cfg.Database), mongoClient, nil
}

func newDMLClient(ctx context.Context, cfg *Config, logger Logger, connectFn ConnectFunc, pingTimeout time.Duration) (*Client, error) {
	const op = "mongodb.newDMLClient"
	clientOpts, err := buildClientOptions(cfg, cfg.Username, cfg.Password, cfg.AuthSource)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	mongoClient, err := connectFn(clientOpts)
	if err != nil {
		return nil, fmt.Errorf("%s: failed to connect: %w", op, err)
	}

	if err := ping(ctx, mongoClient, pingTimeout); err != nil {
		_ = mongoClient.Disconnect(context.Background())
		return nil, fmt.Errorf("%s: ping failed: %w", op, err)
	}

	client := &Client{
		client:   mongoClient,
		config:   cfg,
		logger:   logger,
		Database: mongoClient.Database(cfg.Database),
	}
	logger.Info(op + ": connected to MongoDB")
	return client, nil
}

func ping(ctx context.Context, client MongoClient, timeout time.Duration) error {
	pingCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return client.Ping(pingCtx, readpref.Primary())
}

func buildClientOptions(cfg *Config, username, password, authSource string) (*options.ClientOptions, error) {
	clientOpts := options.Client()

	if hosts := cfg.GetHosts(); len(hosts) > 0 {
		clientOpts.SetHosts(hosts)
	}
	clientOpts.SetMinPoolSize(cfg.MinPoolSize)
	clientOpts.SetMaxPoolSize(cfg.MaxPoolSize)
	clientOpts.SetMaxConnIdleTime(cfg.MaxConnIdleTime)

	if username != "" && password != "" {
		cred := options.Credential{
			Username:   username,
			Password:   password,
			AuthSource: authSource,
		}
		if cfg.AuthMechanism != "" {
			cred.AuthMechanism = cfg.AuthMechanism
		}
		clientOpts.SetAuth(cred)
	}

	clientOpts.SetConnectTimeout(cfg.ConnectTimeout)
	clientOpts.SetServerSelectionTimeout(cfg.ServerSelection)
	clientOpts.SetHeartbeatInterval(cfg.HeartbeatInterval)

	clientOpts.SetRetryWrites(cfg.RetryWrites)
	clientOpts.SetRetryReads(cfg.RetryReads)
	clientOpts.SetReplicaSet(cfg.ReplicaSet)

	tlsConfig, err := cfg.TLS.ToTLSConfig()
	if err != nil {
		return nil, fmt.Errorf("invalid TLS configuration: %w", err)
	}
	if tlsConfig == nil {
		tlsConfig = defaultTLSConfig()
	}
	clientOpts.SetTLSConfig(tlsConfig)

	return clientOpts, nil
}

func (c *Client) Close(ctx context.Context) error {
	const op = "mongodb.Close"
	c.logger.Info(op + ": closing MongoDB connection")
	return c.client.Disconnect(ctx)
}

func (c *Client) CreateIndex(ctx context.Context, collection string, models []mongo.IndexModel, opts ...options.Lister[options.CreateIndexesOptions]) error {
	const op = "mongodb.CreateIndex"
	_, err := c.ddl.Collection(collection).Indexes().CreateMany(ctx, models, opts...)
	if err != nil {
		return fmt.Errorf("%s: failed to create indexes: %w", op, err)
	}
	return nil
}

func (c *Client) RunMigrations(ctx context.Context, name string) error {
	const op = "mongodb.RunMigrations"
	collections, err := c.ddl.ListCollectionNames(ctx, bson.D{})
	if err != nil {
		return fmt.Errorf("%s: failed to list collections: %w", op, err)
	}
	if slices.Contains(collections, name) {
		return ErrCollectionAlreadyExists
	}
	if err := c.ddl.CreateCollection(ctx, name, options.CreateCollection()); err != nil {
		return fmt.Errorf("%s: failed to create collection: %w", op, err)
	}
	return nil
}
