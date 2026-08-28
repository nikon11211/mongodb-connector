package mongodb

import (
	"context"
	"crypto/tls"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/mongo/readpref"
)

var errBoom = errors.New("boom")

type mockLogger struct {
	mock.Mock
}

func (m *mockLogger) DebugF(format string, args ...any) { m.Called(format, args) }
func (m *mockLogger) Debug(msg string)                  { m.Called(msg) }
func (m *mockLogger) Info(msg string)                   { m.Called(msg) }
func (m *mockLogger) Warn(msg string)                   { m.Called(msg) }
func (m *mockLogger) Error(msg string)                  { m.Called(msg) }

type mockMongoClient struct {
	mock.Mock
}

func (m *mockMongoClient) Ping(ctx context.Context, rp *readpref.ReadPref) error {
	args := m.Called(ctx, rp)
	return args.Error(0)
}

func (m *mockMongoClient) Disconnect(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *mockMongoClient) Database(name string, opts ...options.Lister[options.DatabaseOptions]) Database {
	args := m.Called(name, opts)
	return args.Get(0).(Database)
}

type mockDatabase struct {
	mock.Mock
}

func (m *mockDatabase) Collection(name string, opts ...options.Lister[options.CollectionOptions]) Collection {
	args := m.Called(name, opts)
	return args.Get(0).(Collection)
}

func (m *mockDatabase) ListCollectionNames(ctx context.Context, filter any, opts ...options.Lister[options.ListCollectionsOptions]) ([]string, error) {
	args := m.Called(ctx, filter, opts)
	return args.Get(0).([]string), args.Error(1)
}

func (m *mockDatabase) CreateCollection(ctx context.Context, name string, opts ...options.Lister[options.CreateCollectionOptions]) error {
	args := m.Called(ctx, name, opts)
	return args.Error(0)
}

type mockCollection struct {
	mock.Mock
}

func (m *mockCollection) Indexes() Indexes {
	args := m.Called()
	return args.Get(0).(Indexes)
}

type mockIndexes struct {
	mock.Mock
}

func (m *mockIndexes) CreateMany(ctx context.Context, models []mongo.IndexModel, opts ...options.Lister[options.CreateIndexesOptions]) ([]string, error) {
	args := m.Called(ctx, models, opts)
	return args.Get(0).([]string), args.Error(1)
}

func setConnect(fn ConnectFunc) func() {
	prev := connect
	connect = fn
	return func() {
		connect = prev
	}
}

func connectSequence(fns ...func(*options.ClientOptions) (MongoClient, error)) ConnectFunc {
	i := 0
	return func(opts *options.ClientOptions) (MongoClient, error) {
		if i >= len(fns) {
			return nil, errors.New("unexpected connect call")
		}
		fn := fns[i]
		i++
		return fn(opts)
	}
}

func newTestConfig() *Config {
	return &Config{
		Hosts:             "mongodb1:27017, mongodb2:27017",
		ReplicaSet:        "rs0",
		Database:          "appdb",
		Username:          "dml_user",
		Password:          "dml_pass",
		DDLUser:           "ddl_user",
		DDLPassword:       "ddl_pass",
		DDLAuthSource:     "admin",
		AuthSource:        "admin",
		AuthMechanism:     "SCRAM-SHA-256",
		MinPoolSize:       1,
		MaxPoolSize:       10,
		MaxConnIdleTime:   60 * time.Second,
		ConnectTimeout:    5 * time.Second,
		ServerSelection:   30 * time.Second,
		HeartbeatInterval: 10 * time.Second,
		RetryWrites:       true,
		RetryReads:        true,
	}
}

func expectConnectedClient(m *mockMongoClient, db Database) {
	m.On("Ping", mock.Anything, mock.AnythingOfType("*readpref.ReadPref")).Return(nil)
	m.On("Database", "appdb", mock.Anything).Return(db)
}

func TestNew_NilConfig(t *testing.T) {
	logger := new(mockLogger)
	client, err := New(context.Background(), nil, logger)
	require.Error(t, err)
	assert.ErrorContains(t, err, "config cannot be nil")
	assert.Nil(t, client)
	logger.AssertNotCalled(t, "Error")
}

func TestNew_Success(t *testing.T) {
	ddlDB := new(mockDatabase)
	ddlClient := new(mockMongoClient)
	expectConnectedClient(ddlClient, ddlDB)

	dmlDB := new(mockDatabase)
	dmlClient := new(mockMongoClient)
	expectConnectedClient(dmlClient, dmlDB)

	logger := new(mockLogger)
	logger.On("Info", mock.AnythingOfType("string")).Return()

	var gotOpts []*options.ClientOptions
	restore := setConnect(func(opts *options.ClientOptions) (MongoClient, error) {
		gotOpts = append(gotOpts, opts)
		switch len(gotOpts) {
		case 1:
			return ddlClient, nil
		default:
			return dmlClient, nil
		}
	})
	defer restore()

	client, err := New(context.Background(), newTestConfig(), logger)
	require.NoError(t, err)
	require.NotNil(t, client)

	assert.Same(t, dmlDB, client.Database)
	assert.Same(t, ddlDB, client.ddl)
	assert.Same(t, dmlClient, client.client)
	assert.NotNil(t, client.config)
	assert.Same(t, logger, client.logger)

	require.Len(t, gotOpts, 2)
	assert.Equal(t, []string{"mongodb1:27017", "mongodb2:27017"}, gotOpts[0].Hosts)
	require.NotNil(t, gotOpts[0].Auth)
	assert.Equal(t, "ddl_user", gotOpts[0].Auth.Username)
	assert.Equal(t, "ddl_pass", gotOpts[0].Auth.Password)
	assert.Equal(t, "SCRAM-SHA-256", gotOpts[0].Auth.AuthMechanism)
	require.NotNil(t, gotOpts[1].Auth)
	assert.Equal(t, "dml_user", gotOpts[1].Auth.Username)
	require.NotNil(t, gotOpts[0].ReplicaSet)
	assert.Equal(t, "rs0", *gotOpts[0].ReplicaSet)
	require.NotNil(t, gotOpts[0].MinPoolSize)
	assert.Equal(t, uint64(1), *gotOpts[0].MinPoolSize)
	require.NotNil(t, gotOpts[0].MaxPoolSize)
	assert.Equal(t, uint64(10), *gotOpts[0].MaxPoolSize)
	require.NotNil(t, gotOpts[0].RetryWrites)
	assert.True(t, *gotOpts[0].RetryWrites)
	require.NotNil(t, gotOpts[0].TLSConfig)
	assert.Equal(t, uint16(tls.VersionTLS12), gotOpts[0].TLSConfig.MinVersion)

	ddlClient.AssertExpectations(t)
	dmlClient.AssertExpectations(t)
	logger.AssertExpectations(t)
}

func TestNew_DDLBuildOptionsError(t *testing.T) {
	logger := new(mockLogger)
	logger.On("Error", mock.AnythingOfType("string")).Return()

	cfg := newTestConfig()
	cfg.TLS = &TLSConfig{Enabled: true, CAFile: "/nonexistent/ca.pem"}

	restore := setConnect(func(*options.ClientOptions) (MongoClient, error) {
		t.Fatal("connect must not be called")
		return nil, nil
	})
	defer restore()

	client, err := New(context.Background(), cfg, logger)
	require.Error(t, err)
	assert.ErrorContains(t, err, "invalid TLS configuration")
	assert.Nil(t, client)
	logger.AssertExpectations(t)
}

func TestNew_DDLConnectError(t *testing.T) {
	logger := new(mockLogger)
	logger.On("Error", mock.AnythingOfType("string")).Return()

	restore := setConnect(func(*options.ClientOptions) (MongoClient, error) {
		return nil, errBoom
	})
	defer restore()

	client, err := New(context.Background(), newTestConfig(), logger)
	require.Error(t, err)
	assert.ErrorIs(t, err, errBoom)
	assert.Nil(t, client)
	logger.AssertExpectations(t)
}

func TestNew_DDLPingError(t *testing.T) {
	ddlClient := new(mockMongoClient)
	ddlClient.On("Ping", mock.Anything, mock.AnythingOfType("*readpref.ReadPref")).Return(errBoom)
	ddlClient.On("Disconnect", mock.Anything).Return(nil)

	logger := new(mockLogger)
	logger.On("Error", mock.AnythingOfType("string")).Return()

	restore := setConnect(func(*options.ClientOptions) (MongoClient, error) {
		return ddlClient, nil
	})
	defer restore()

	client, err := New(context.Background(), newTestConfig(), logger)
	require.Error(t, err)
	assert.ErrorIs(t, err, errBoom)
	assert.Nil(t, client)
	ddlClient.AssertExpectations(t)
	logger.AssertExpectations(t)
}

func TestNew_DMLConnectError(t *testing.T) {
	ddlDB := new(mockDatabase)
	ddlClient := new(mockMongoClient)
	expectConnectedClient(ddlClient, ddlDB)
	ddlClient.On("Disconnect", mock.Anything).Return(nil)

	logger := new(mockLogger)
	logger.On("Error", mock.AnythingOfType("string")).Return()

	restore := setConnect(connectSequence(
		func(*options.ClientOptions) (MongoClient, error) { return ddlClient, nil },
		func(*options.ClientOptions) (MongoClient, error) { return nil, errBoom },
	))
	defer restore()

	client, err := New(context.Background(), newTestConfig(), logger)
	require.Error(t, err)
	assert.ErrorIs(t, err, errBoom)
	assert.Nil(t, client)
	ddlClient.AssertExpectations(t)
	logger.AssertExpectations(t)
}

func TestNew_DMLPingError(t *testing.T) {
	ddlDB := new(mockDatabase)
	ddlClient := new(mockMongoClient)
	expectConnectedClient(ddlClient, ddlDB)
	ddlClient.On("Disconnect", mock.Anything).Return(nil)

	dmlClient := new(mockMongoClient)
	dmlClient.On("Ping", mock.Anything, mock.AnythingOfType("*readpref.ReadPref")).Return(errBoom)
	dmlClient.On("Disconnect", mock.Anything).Return(nil)

	logger := new(mockLogger)
	logger.On("Error", mock.AnythingOfType("string")).Return()

	restore := setConnect(connectSequence(
		func(*options.ClientOptions) (MongoClient, error) { return ddlClient, nil },
		func(*options.ClientOptions) (MongoClient, error) { return dmlClient, nil },
	))
	defer restore()

	client, err := New(context.Background(), newTestConfig(), logger)
	require.Error(t, err)
	assert.ErrorIs(t, err, errBoom)
	assert.Nil(t, client)
	ddlClient.AssertExpectations(t)
	dmlClient.AssertExpectations(t)
	logger.AssertExpectations(t)
}

func TestNew_WithConnectOption(t *testing.T) {
	ddlDB := new(mockDatabase)
	ddlClient := new(mockMongoClient)
	expectConnectedClient(ddlClient, ddlDB)

	dmlDB := new(mockDatabase)
	dmlClient := new(mockMongoClient)
	expectConnectedClient(dmlClient, dmlDB)

	calls := 0
	fn := func(*options.ClientOptions) (MongoClient, error) {
		calls++
		if calls == 1 {
			return ddlClient, nil
		}
		return dmlClient, nil
	}

	client, err := New(context.Background(), newTestConfig(), NoopLogger{}, WithConnect(fn))
	require.NoError(t, err)
	require.NotNil(t, client)
	assert.Equal(t, 2, calls)
	ddlClient.AssertExpectations(t)
	dmlClient.AssertExpectations(t)
}

func TestNew_WithPingTimeout(t *testing.T) {
	withDeadline := mock.MatchedBy(func(ctx context.Context) bool {
		_, ok := ctx.Deadline()
		return ok
	})

	ddlClient := new(mockMongoClient)
	ddlClient.On("Ping", withDeadline, mock.AnythingOfType("*readpref.ReadPref")).Return(nil)
	ddlClient.On("Database", "appdb", mock.Anything).Return(new(mockDatabase))

	dmlClient := new(mockMongoClient)
	dmlClient.On("Ping", withDeadline, mock.AnythingOfType("*readpref.ReadPref")).Return(nil)
	dmlClient.On("Database", "appdb", mock.Anything).Return(new(mockDatabase))

	restore := setConnect(connectSequence(
		func(*options.ClientOptions) (MongoClient, error) { return ddlClient, nil },
		func(*options.ClientOptions) (MongoClient, error) { return dmlClient, nil },
	))
	defer restore()

	client, err := New(context.Background(), newTestConfig(), NoopLogger{}, WithPingTimeout(2*time.Second))
	require.NoError(t, err)
	require.NotNil(t, client)
	ddlClient.AssertExpectations(t)
	dmlClient.AssertExpectations(t)
}

func TestNewInternal_BuildOptionsError(t *testing.T) {
	cfg := newTestConfig()
	cfg.TLS = &TLSConfig{Enabled: true, CAFile: "/nonexistent/ca.pem"}

	_, err := newDMLClient(context.Background(), cfg, NoopLogger{}, func(*options.ClientOptions) (MongoClient, error) {
		t.Fatal("connect must not be called")
		return nil, nil
	}, defaultPingTimeout)
	require.Error(t, err)
	assert.ErrorContains(t, err, "invalid TLS configuration")
}

func TestClient_Close(t *testing.T) {
	m := new(mockMongoClient)
	m.On("Disconnect", mock.Anything).Return(nil)

	logger := new(mockLogger)
	logger.On("Info", mock.AnythingOfType("string")).Return()

	c := &Client{client: m, logger: logger}
	require.NoError(t, c.Close(context.Background()))
	m.AssertExpectations(t)
	logger.AssertExpectations(t)
}

func TestClient_CloseError(t *testing.T) {
	m := new(mockMongoClient)
	m.On("Disconnect", mock.Anything).Return(errBoom)

	c := &Client{client: m, logger: NoopLogger{}}
	require.ErrorIs(t, c.Close(context.Background()), errBoom)
	m.AssertExpectations(t)
}

func TestClient_CreateIndex(t *testing.T) {
	models := []mongo.IndexModel{{Keys: bson.D{{Key: "user_id", Value: 1}}}}

	idx := new(mockIndexes)
	idx.On("CreateMany", mock.Anything, models, mock.Anything).Return([]string{"user_id_1"}, nil)

	coll := new(mockCollection)
	coll.On("Indexes").Return(idx)

	db := new(mockDatabase)
	db.On("Collection", "users", mock.Anything).Return(coll)

	c := &Client{ddl: db, logger: NoopLogger{}}
	require.NoError(t, c.CreateIndex(context.Background(), "users", models))

	db.AssertExpectations(t)
	coll.AssertExpectations(t)
	idx.AssertExpectations(t)
}

func TestClient_CreateIndexError(t *testing.T) {
	models := []mongo.IndexModel{{Keys: bson.D{{Key: "user_id", Value: 1}}}}

	idx := new(mockIndexes)
	idx.On("CreateMany", mock.Anything, models, mock.Anything).Return([]string(nil), errBoom)

	coll := new(mockCollection)
	coll.On("Indexes").Return(idx)

	db := new(mockDatabase)
	db.On("Collection", "users", mock.Anything).Return(coll)

	c := &Client{ddl: db, logger: NoopLogger{}}
	err := c.CreateIndex(context.Background(), "users", models)
	require.ErrorIs(t, err, errBoom)
	assert.ErrorContains(t, err, "failed to create indexes")

	db.AssertExpectations(t)
	coll.AssertExpectations(t)
	idx.AssertExpectations(t)
}

func TestClient_RunMigrations_ListError(t *testing.T) {
	db := new(mockDatabase)
	db.On("ListCollectionNames", mock.Anything, mock.Anything, mock.Anything).Return([]string(nil), errBoom)

	c := &Client{ddl: db, logger: NoopLogger{}}
	err := c.RunMigrations(context.Background(), "orders")
	require.ErrorIs(t, err, errBoom)
	assert.ErrorContains(t, err, "failed to list collections")
	db.AssertExpectations(t)
}

func TestClient_RunMigrations_AlreadyExists(t *testing.T) {
	db := new(mockDatabase)
	db.On("ListCollectionNames", mock.Anything, mock.Anything, mock.Anything).Return([]string{"orders", "users"}, nil)

	c := &Client{ddl: db, logger: NoopLogger{}}
	err := c.RunMigrations(context.Background(), "orders")
	require.ErrorIs(t, err, ErrCollectionAlreadyExists)
	db.AssertExpectations(t)
}

func TestClient_RunMigrations_Success(t *testing.T) {
	db := new(mockDatabase)
	db.On("ListCollectionNames", mock.Anything, mock.Anything, mock.Anything).Return([]string{"users"}, nil)
	db.On("CreateCollection", mock.Anything, "orders", mock.Anything).Return(nil)

	c := &Client{ddl: db, logger: NoopLogger{}}
	require.NoError(t, c.RunMigrations(context.Background(), "orders"))
	db.AssertExpectations(t)
}

func TestClient_RunMigrations_CreateError(t *testing.T) {
	db := new(mockDatabase)
	db.On("ListCollectionNames", mock.Anything, mock.Anything, mock.Anything).Return([]string{"users"}, nil)
	db.On("CreateCollection", mock.Anything, "orders", mock.Anything).Return(errBoom)

	c := &Client{ddl: db, logger: NoopLogger{}}
	err := c.RunMigrations(context.Background(), "orders")
	require.ErrorIs(t, err, errBoom)
	assert.ErrorContains(t, err, "failed to create collection")
	db.AssertExpectations(t)
}

func TestDefaultConnect(t *testing.T) {
	ctx := context.Background()
	client, err := connect(options.Client().SetConnectTimeout(100 * time.Millisecond))
	require.NoError(t, err)
	require.NotNil(t, client)
	require.NoError(t, client.Disconnect(ctx))
}

func TestDefaultConnectError(t *testing.T) {
	opts := options.Client().SetAuth(options.Credential{
		Username:      "user",
		Password:      "pass",
		AuthMechanism: "MONGODB-CR",
	})
	client, err := connect(opts)
	require.Error(t, err)
	assert.Nil(t, client)
}

func TestAdaptersWithRealDriver(t *testing.T) {
	ctx := context.Background()
	opts := options.Client().
		SetHosts([]string{"127.0.0.1:1"}).
		SetConnectTimeout(250 * time.Millisecond).
		SetServerSelectionTimeout(250 * time.Millisecond).
		SetTimeout(250 * time.Millisecond)

	raw, err := mongo.Connect(opts)
	require.NoError(t, err)
	defer func() { _ = raw.Disconnect(ctx) }()

	adapter := &clientAdapter{c: raw}

	require.Error(t, adapter.Ping(ctx, readpref.Primary()))

	db := adapter.Database("testdb")
	require.NotNil(t, db)

	_, err = db.ListCollectionNames(ctx, bson.D{})
	require.Error(t, err)

	require.Error(t, db.CreateCollection(ctx, "testcoll"))

	coll := db.Collection("testcoll")
	require.NotNil(t, coll)

	indexes := coll.Indexes()
	require.NotNil(t, indexes)

	_, err = indexes.CreateMany(ctx, []mongo.IndexModel{{Keys: bson.D{{Key: "a", Value: 1}}}})
	require.Error(t, err)

	require.NoError(t, adapter.Disconnect(ctx))
}
