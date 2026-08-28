<p align="center">
  <h1 align="center">MongoDB Connector - Enterprise-Grade MongoDB Client Library for Go</h1>
</p>

<p align="center">
  <a href="https://pkg.go.dev/github.com/nikon11211/mongodb-connector">
    <img src="https://pkg.go.dev/badge/github.com/nikon11211/mongodb-connector.svg" alt="Go Reference"/>
  </a>
  <a href="https://goreportcard.com/report/github.com/nikon11211/mongodb-connector">
    <img src="https://goreportcard.com/badge/github.com/nikon11211/mongodb-connector" alt="Go Report Card"/>
  </a>
  <a href="https://github.com/nikon11211/mongodb-connector/actions/workflows/test.yaml">
    <img src="https://github.com/nikon11211/mongodb-connector/actions/workflows/test.yaml/badge.svg" alt="Tests"/>
  </a>
  <a href="https://codecov.io/gh/nikon11211/mongodb-connector">
    <img src="https://codecov.io/gh/nikon11211/mongodb-connector/branch/main/graph/badge.svg" alt="Coverage"/>
  </a>
  <a href="https://sonarcloud.io/summary/overall?id=nikon11211_mongodb-connector">
    <img src="https://sonarcloud.io/api/project_badges/measure?project=nikon11211_mongodb-connector&metric=coverage" alt="SonarCloud Coverage"/>
  </a>
  <a href="https://opensource.org/licenses/MIT">
    <img src="https://img.shields.io/badge/License-MIT-yellow.svg" alt="License: MIT"/>
  </a>
  <a href="https://golang.org/">
    <img src="https://img.shields.io/badge/Go-%3E%3D%201.26-blue" alt="Go Version"/>
  </a>
</p>

<p align="center">
  <b>A production-ready MongoDB client library for Go microservices</b><br/>
  <i>mongo-driver v2 • Separate DDL/DML credentials • TLS • Migrations • 100% test coverage</i>
</p>

---

## ✨ Why MongoDB Connector?

This library wraps the official `mongo-driver` v2 with production concerns already solved: **separate DDL and DML credentials** (schema changes never run with the application account), automatic collection migrations, index management, TLS with custom CAs, retryable reads/writes, and sane pooling defaults — all behind a small, opinionated API.

```go
// One call to get a fully verified, dual-connection client
client, err := mongodb.New(ctx, cfg, logger)
if err != nil {
    log.Fatal(err)
}
defer client.Close(ctx)

// Migrations and indexes run with DDL credentials...
client.RunMigrations(ctx, "orders")
client.CreateIndex(ctx, "orders", []mongo.IndexModel{{Keys: bson.D{{Key: "customer_id", Value: 1}}}})
```

---

## 🎯 Features

<table>
<tr>
<td width="50%">

### 🚀 Core Features
- **Dual-connection design** - DDL and DML credentials in a single client
- **Primary read-preference ping** on every connection at startup
- **RunMigrations** - idempotent collection creation with `ErrCollectionAlreadyExists`
- **CreateIndex** - index management on the DDL connection
- **Host list** - comma-separated hosts parsed into a cluster seed list
- **Retryable reads/writes** and configurable pool sizes

### 🔒 Security
- TLS with custom CA bundle and client certificates
- `InsecureSkipVerify` escape hatch for private networks
- Default TLS 1.2 minimum version
- Authentication source and mechanism (e.g. SCRAM-SHA-256)

</td>
<td width="50%">

### 🧩 Testability
- Narrow interfaces (`MongoClient`, `Database`, `Collection`, `Indexes`) satisfied by the real driver
- Injectable `connect` factory - mocks instead of a real MongoDB in unit tests
- **100.0% statement coverage** excluding examples
- Race-detector clean (`go test -race`)

### 📦 Developer Experience
- Zero corporate or private dependencies
- Logger-agnostic via a 5-method `Logger` interface (`NoopLogger` included)
- Functional options (`WithPingTimeout`, `WithConnect`)
- English error messages with `%w` wrapping
- Benchmarks and runnable examples

</td>
</tr>
</table>

## 📦 Installation

```bash
go get github.com/nikon11211/mongodb-connector
```

## 🏗️ Architecture

```
┌────────────────────────────────────────────────────────────────┐
│                        Your Application                        │
├────────────────────────────────────────────────────────────────┤
│                    mongodb.New(ctx, cfg, logger)               │
│                                                                │
│  ┌───────────────────────────┐   ┌──────────────────────────┐  │
│  │  DDL connection           │   │  DML connection          │  │
│  │  (DDLUser/DDLPassword)    │   │  (Username/Password)     │  │
│  │  ping ✓ readpref.Primary  │   │  ping ✓ readpref.Primary │  │
│  ├───────────────────────────┤   ├──────────────────────────┤  │
│  │  RunMigrations            │   │  embedded Database handle│  │
│  │  CreateIndex              │   │  (your app queries)      │  │
│  └───────────────────────────┘   └──────────────────────────┘  │
│                          │                                     │
│                   mongo-driver v2                              │
└────────────────────────────────────────────────────────────────┘
```

The `Client` embeds the DML `Database` handle, so `client.Collection("orders").Find(...)` works right out of the box. Schema operations (`RunMigrations`, `CreateIndex`) are always routed to the DDL connection.

## 🚀 Quick Start

```go
package main

import (
    "context"
    "log"
    "time"

    "github.com/nikon11211/mongodb-connector"
    "go.mongodb.org/mongo-driver/v2/bson"
    "go.mongodb.org/mongo-driver/v2/mongo"
    "go.mongodb.org/mongo-driver/v2/mongo/options"
)

func main() {
    ctx := context.Background()

    cfg := &mongodb.Config{
        Hosts:          "mongo1:27017, mongo2:27017, mongo3:27017",
        ReplicaSet:     "rs0",
        Database:       "orders",
        Username:       "app_user",
        Password:       "app_pass",
        DDLUser:        "ddl_user",
        DDLPassword:    "ddl_pass",
        AuthSource:     "admin",
        MaxPoolSize:    20,
        ConnectTimeout: 5 * time.Second,
        RetryWrites:    true,
        RetryReads:     true,
        TLS: &mongodb.TLSConfig{
            Enabled:            true,
            CAFile:             "/etc/ssl/mongodb-ca.pem",
            CertKeyFile:        "/etc/ssl/mongodb-client.pem",
            InsecureSkipVerify: false,
        },
    }

    client, err := mongodb.New(ctx, cfg, mongodb.NoopLogger{})
    if err != nil {
        log.Fatalf("failed to connect: %v", err)
    }
    defer client.Close(ctx)

    if err := client.RunMigrations(ctx, "orders"); err != nil {
        log.Fatalf("migration failed: %v", err)
    }

    if err := client.CreateIndex(ctx, "orders", []mongo.IndexModel{
        {Keys: bson.D{{Key: "customer_id", Value: 1}}, Options: options.Index().SetUnique(true)},
    }); err != nil {
        log.Fatalf("failed to create index: %v", err)
    }

    // Query through the embedded DML database handle.
    var doc bson.M
    err = client.Collection("orders").FindOne(ctx, bson.D{{Key: "customer_id", Value: 42}}).Decode(&doc)
    // ...
}
```

A complete runnable example lives in [`examples/basic/main.go`](examples/basic/main.go).

## 🔧 Configuration Reference

```go
type Config struct {
    URI               string         // MongoDB connection string (optional)
    Hosts             string         // Comma-separated hosts, e.g. "mongo1:27017, mongo2:27017"
    ReplicaSet        string         // Replica set name
    Database          string         // Target database
    Username          string         // DML user (application account)
    Password          string         // DML user password
    DDLUser           string         // DDL user (schema management account)
    DDLPassword       string         // DDL user password
    DDLAuthSource     string         // Auth database of the DDL user
    AuthSource        string         // Auth database of the DML user
    AuthMechanism     string         // e.g. "SCRAM-SHA-256"
    MinPoolSize       uint64         // Minimum connection pool size
    MaxPoolSize       uint64         // Maximum connection pool size
    MaxConnIdleTime   time.Duration  // Max idle time for pooled connections
    ConnectTimeout    time.Duration  // Connection establishment timeout
    SocketTimeout     time.Duration  // Reserved for compatibility (unused by driver v2)
    ServerSelection   time.Duration  // Server selection timeout
    HeartbeatInterval time.Duration  // Heartbeat interval
    RetryWrites       bool           // Enable retryable writes
    RetryReads        bool           // Enable retryable reads
    TLS               *TLSConfig     // TLS settings (nil disables TLS)
}

type TLSConfig struct {
    Enabled            bool   // Enable TLS
    CAFile             string // Path to a PEM CA bundle
    CertKeyFile        string // Path to a PEM file with client cert + key
    InsecureSkipVerify bool   // Skip server certificate verification
}
```

### Errors

| Error | Description |
|-------|-------------|
| `ErrCollectionAlreadyExists` | `RunMigrations` was called for an existing collection |

### Options

```go
mongodb.New(ctx, cfg, logger,
    mongodb.WithPingTimeout(10*time.Second), // connectivity-check timeout
    mongodb.WithConnect(customConnect),      // override the connect factory (testing)
)
```

## 🧪 Testing

```go
// Run all tests (100.0% coverage, excluding examples)
go test -race -coverprofile=coverage.txt -covermode=atomic $(go list ./... | grep -v /examples)

// Coverage report
go tool cover -func=coverage.txt
go tool cover -html=coverage.txt

// Benchmarks
go test -bench=. -benchmem -run '^$' .
```

| Benchmark                   | What it measures                          |
|-----------------------------|-------------------------------------------|
| `BenchmarkGetHosts`         | Comma-separated host list parsing         |
| `BenchmarkTLSConfigToTLSConfig` | Internal TLS config conversion        |

The unit tests use `testify/mock` mocks against the library's narrow interfaces — no real MongoDB required. A dedicated test also exercises the driver adapters against a real (lazy) `mongo-driver` client.

## 🤝 Contributing

We welcome contributions! Here's how you can help:

1. **Fork** the repository
2. **Create** a feature branch (`git checkout -b feature/amazing-feature`)
3. **Commit** your changes (`git commit -m 'Add amazing feature'`)
4. **Push** to the branch (`git push origin feature/amazing-feature`)
5. **Open** a Pull Request

Please keep the test suite at 100.0% statement coverage.

## 📄 License

MIT License - see [LICENSE](LICENSE) for details.

## 🌟 Show Your Support

Give a ⭐️ if this project helped you! Share it with your team to simplify MongoDB integration in your Go services.

## 🙏 Acknowledgments

- [MongoDB Go Driver](https://github.com/mongodb/mongo-go-driver) - The official MongoDB Go driver (v2)
- [testify](https://github.com/stretchr/testify) - Mocks and assertions
- [MongoDB](https://www.mongodb.com/) - The document database this library connects to
- The Go community for feedback and best practices

---

<p align="center">
  <b>Made with ❤️ for the Go community</b><br/>
  <sub>Built for reliability, tested to 100%</sub>
</p>
