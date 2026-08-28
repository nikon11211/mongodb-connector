package mongodb

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const defaultPingTimeout = 5 * time.Second

type ConnectFunc func(opts *options.ClientOptions) (MongoClient, error)

var connect ConnectFunc = func(opts *options.ClientOptions) (MongoClient, error) {
	client, err := mongo.Connect(opts)
	if err != nil {
		return nil, err
	}
	return &clientAdapter{c: client}, nil
}

type ClientOption func(*clientOptions)

type clientOptions struct {
	pingTimeout time.Duration
	connect     ConnectFunc
}

func defaultClientOptions() *clientOptions {
	return &clientOptions{
		pingTimeout: defaultPingTimeout,
	}
}

func WithPingTimeout(d time.Duration) ClientOption {
	return func(o *clientOptions) {
		o.pingTimeout = d
	}
}

func WithConnect(fn ConnectFunc) ClientOption {
	return func(o *clientOptions) {
		o.connect = fn
	}
}
