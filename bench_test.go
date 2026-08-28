package mongodb

import (
	"testing"
)

func BenchmarkGetHosts(b *testing.B) {
	cfg := &Config{Hosts: "mongodb1:27017, mongodb2:27017, mongodb3:27017 "}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cfg.GetHosts()
	}
}

func BenchmarkTLSConfigToTLSConfig(b *testing.B) {
	tlsCfg := &TLSConfig{Enabled: true, InsecureSkipVerify: true}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = tlsCfg.ToTLSConfig()
	}
}
