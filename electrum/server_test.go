package electrum_test

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zauberhaus/go-electrum/electrum"
	"github.com/zauberhaus/logger"
)

func TestPing(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client, transport := newTestClient(ctx, t)
	defer client.Shutdown()

	go func() {
		<-transport.sent()

		jsonResp, err := json.Marshal(map[string]interface{}{"jsonrpc": "2.0", "id": 1, "result": nil})
		require.NoError(t, err)

		transport.responses() <- jsonResp
	}()

	err := client.Ping(ctx)
	require.NoError(t, err)
}

func TestServerAddPeer(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client, transport := newTestClient(ctx, t)
	defer client.Shutdown()

	features := &electrum.ServerFeaturesResult{}

	go func() {
		<-transport.sent()

		jsonResp, err := json.Marshal(map[string]interface{}{"jsonrpc": "2.0", "id": 1, "result": "ok"})
		require.NoError(t, err)

		transport.responses() <- jsonResp
	}()

	err := client.ServerAddPeer(ctx, features)
	require.NoError(t, err)
}

func TestServerBanner(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client, transport := newTestClient(ctx, t)
	defer client.Shutdown()

	expectedBanner := "Welcome to Electrum!"

	go func() {
		<-transport.sent()

		jsonResp, err := json.Marshal(map[string]interface{}{"jsonrpc": "2.0", "id": 1, "result": expectedBanner})
		require.NoError(t, err)

		transport.responses() <- jsonResp
	}()

	banner, err := client.ServerBanner(ctx)
	require.NoError(t, err)
	assert.Equal(t, expectedBanner, banner)
}

func TestServerDonation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client, transport := newTestClient(ctx, t)
	defer client.Shutdown()

	expectedAddress := "bc1q..."

	go func() {
		<-transport.sent()

		jsonResp, err := json.Marshal(map[string]interface{}{"jsonrpc": "2.0", "id": 1, "result": expectedAddress})
		require.NoError(t, err)

		transport.responses() <- jsonResp
	}()

	address, err := client.ServerDonation(ctx)
	require.NoError(t, err)
	assert.Equal(t, expectedAddress, address)
}

func TestServerFeatures(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client, transport := newTestClient(ctx, t)
	defer client.Shutdown()

	expectedFeatures := &electrum.ServerFeaturesResult{
		GenesisHash: "genesis",
		Hosts: map[string]electrum.Host{
			"host1": {TCPPort: 50001, SSLPort: 50002},
		},
		ProtocolMax:   "1.4",
		ProtocolMin:   "1.4",
		ServerVersion: "ElectrumX 1.16.0",
		HashFunction:  "sha256",
	}

	go func() {
		<-transport.sent()

		jsonResp, err := json.Marshal(map[string]interface{}{"jsonrpc": "2.0", "id": 1, "result": expectedFeatures})
		require.NoError(t, err)

		transport.responses() <- jsonResp
	}()

	features, err := client.ServerFeatures(ctx)
	require.NoError(t, err)
	if !reflect.DeepEqual(expectedFeatures, features) {
		t.Errorf("Expected features %v, got %v", expectedFeatures, features)
	}
}

func TestServerPeers(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	log := logger.GetLogger(ctx)
	//log.EnableDebug()
	log = log.AddSkip(-1)
	ctx = logger.AddLogger(ctx, log)

	client, err := electrum.NewClientTCP(context.Background(), "bch.imaginary.cash:50001")
	require.NoError(t, err)

	defer client.Shutdown()

	peers, err := client.ServerPeers(ctx)
	assert.NoError(t, err)
	assert.NotEmpty(t, peers)
}

func TestServerPeersMock(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client, transport := newTestClient(ctx, t)
	defer client.Shutdown()

	result := [][]interface{}{
		{"peer1", "v1.4", []interface{}{"p1"}},
	}

	expectedPeers := []*electrum.Peer{
		{"peer1", "v1.4", []string{"p1"}},
	}

	go func() {
		<-transport.sent()

		jsonResp, err := json.Marshal(map[string]interface{}{"jsonrpc": "2.0", "id": 1, "result": result})
		require.NoError(t, err)

		transport.responses() <- jsonResp
	}()

	peers, err := client.ServerPeers(ctx)
	require.NoError(t, err)
	assert.Equal(t, expectedPeers, peers)
}

func TestServerVersion(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client, transport := newTestClient(ctx, t)
	defer client.Shutdown()

	expectedServerVer := "ElectrumX 1.16.0"
	expectedProtocolVer := "1.4"

	go func() {
		<-transport.sent()

		jsonResp, err := json.Marshal(map[string]interface{}{"jsonrpc": "2.0", "id": 1, "result": [2]string{expectedServerVer, expectedProtocolVer}})
		require.NoError(t, err)

		transport.responses() <- jsonResp
	}()

	serverVer, protocolVer, err := client.ServerVersion(ctx)
	require.NoError(t, err)
	assert.Equal(t, expectedServerVer, serverVer)
	assert.Equal(t, expectedProtocolVer, protocolVer)
}
