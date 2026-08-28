package mongodb

import (
	"strings"
	"time"
)

type Config struct {
	URI               string        `mapstructure:"uri"`
	Hosts             string        `mapstructure:"hosts"`
	ReplicaSet        string        `mapstructure:"replica_set"`
	Database          string        `mapstructure:"database"`
	Username          string        `mapstructure:"username"`
	DDLUser           string        `mapstructure:"ddl_username"`
	Password          string        `mapstructure:"password"`
	DDLPassword       string        `mapstructure:"ddl_password"`
	DDLAuthSource     string        `mapstructure:"ddl_auth_source"`
	AuthSource        string        `mapstructure:"auth_source"`
	AuthMechanism     string        `mapstructure:"auth_mechanism"`
	MinPoolSize       uint64        `mapstructure:"min_pool_size"`
	MaxPoolSize       uint64        `mapstructure:"max_pool_size"`
	MaxConnIdleTime   time.Duration `mapstructure:"max_conn_idle_time"`
	ConnectTimeout    time.Duration `mapstructure:"connect_timeout"`
	SocketTimeout     time.Duration `mapstructure:"socket_timeout"`
	ServerSelection   time.Duration `mapstructure:"server_selection_timeout"`
	HeartbeatInterval time.Duration `mapstructure:"heartbeat_interval"`
	RetryWrites       bool          `mapstructure:"retry_writes"`
	RetryReads        bool          `mapstructure:"retry_reads"`
	TLS               *TLSConfig    `mapstructure:"tls"`
}

func (c *Config) GetHosts() []string {
	if c.Hosts == "" {
		return nil
	}

	hosts := strings.Split(c.Hosts, ",")
	for i, host := range hosts {
		hosts[i] = strings.TrimSpace(host)
	}
	return hosts
}
