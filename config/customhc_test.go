package config

import (
	"github.com/stretchr/testify/assert"
	"net/http"
	"testing"
)

func simpleHC(endpoint string, transport *http.Transport) <-chan bool {
	statusChannel := make(chan bool)

	go func() {
		statusChannel <- true
	}()

	return statusChannel
}

func TestCustomHCNoFunction(t *testing.T) {
	kvs := BuildKVStoreTestConfig(t)
	err := RegisterHealthCheckForServer(kvs, "not a server name", nil)
	if assert.NotNil(t, err) {
		assert.Equal(t, err, ErrNoHealthCheckFn)
	}
}

func TestCustomHCNoSuchServer(t *testing.T) {
	kvs := BuildKVStoreTestConfig(t)
	err := RegisterHealthCheckForServer(kvs, "not a server name", simpleHC)
	if assert.NotNil(t, err) {
		assert.Equal(t, err, ErrNoSuchServer)
	}
}

func TestCustomHCLookup(t *testing.T) {
	kvs := BuildKVStoreTestConfig(t)
	var hc1 HealthCheckFn = simpleHC
	err := RegisterHealthCheckForServer(kvs, "server1", hc1)

	if assert.Nil(t, err) {
		hcfn := HealthCheckForServer("server1")
		assert.NotNil(t, hcfn)
	}
}
