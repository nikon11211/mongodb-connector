package mongodb

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetHosts_Empty(t *testing.T) {
	cfg := &Config{}
	assert.Nil(t, cfg.GetHosts())
}

func TestGetHosts_Single(t *testing.T) {
	cfg := &Config{Hosts: "mongodb1.example.com:27017"}
	assert.Equal(t, []string{"mongodb1.example.com:27017"}, cfg.GetHosts())
}

func TestGetHosts_MultipleWithSpaces(t *testing.T) {
	cfg := &Config{Hosts: "mongodb1.example.com:27017, mongodb2.example.com:27017,mongodb3.example.com:27017 "}
	assert.Equal(t, []string{
		"mongodb1.example.com:27017",
		"mongodb2.example.com:27017",
		"mongodb3.example.com:27017",
	}, cfg.GetHosts())
}
