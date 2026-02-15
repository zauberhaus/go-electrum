package electrum_test

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zauberhaus/go-electrum/electrum"
)

func TestGetBalance(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client, transport := newTestClient(ctx, t)
	defer client.Shutdown()

	scripthash := "scripthash"
	expectedBalance := electrum.GetBalanceResult{
		Confirmed:   1.23,
		Unconfirmed: 4.56,
	}

	go func() {
		<-transport.sent()

		resp := electrum.GetBalanceResp{
			Result: expectedBalance,
		}
		jsonResp, err := json.Marshal(map[string]interface{}{"jsonrpc": "2.0", "id": 1, "result": resp.Result})
		require.NoError(t, err)

		transport.responses() <- jsonResp
	}()

	balance, err := client.GetBalance(ctx, scripthash)
	require.NoError(t, err)
	assert.Equal(t, expectedBalance, balance)
}

func TestGetHistory(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client, transport := newTestClient(ctx, t)
	defer client.Shutdown()

	scripthash := "scripthash"
	expectedHistory := []*electrum.GetMempoolResult{
		{Hash: "hash1", Height: 123},
		{Hash: "hash2", Height: 456},
	}

	go func() {
		<-transport.sent()

		resp := electrum.GetMempoolResp{
			Result: expectedHistory,
		}
		jsonResp, err := json.Marshal(map[string]interface{}{"jsonrpc": "2.0", "id": 1, "result": resp.Result})
		require.NoError(t, err)

		transport.responses() <- jsonResp
	}()

	history, err := client.GetHistory(ctx, scripthash)
	require.NoError(t, err)
	if !reflect.DeepEqual(expectedHistory, history) {
		t.Errorf("Expected history %v, got %v", expectedHistory, history)
	}
}

func TestGetMempool(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client, transport := newTestClient(ctx, t)
	defer client.Shutdown()

	scripthash := "scripthash"
	expectedMempool := []*electrum.GetMempoolResult{
		{Hash: "hash1", Fee: 123},
		{Hash: "hash2", Fee: 456},
	}

	go func() {
		<-transport.sent()

		resp := electrum.GetMempoolResp{
			Result: expectedMempool,
		}
		jsonResp, err := json.Marshal(map[string]interface{}{"jsonrpc": "2.0", "id": 1, "result": resp.Result})
		require.NoError(t, err)

		transport.responses() <- jsonResp
	}()

	mempool, err := client.GetMempool(ctx, scripthash)
	require.NoError(t, err)
	if !reflect.DeepEqual(expectedMempool, mempool) {
		t.Errorf("Expected mempool %v, got %v", expectedMempool, mempool)
	}
}

func TestListUnspent(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client, transport := newTestClient(ctx, t)
	defer client.Shutdown()

	scripthash := "scripthash"
	expectedUTXOs := []*electrum.ListUnspentResult{
		{Height: 123, Position: 1, Hash: "hash1", Value: 100},
		{Height: 456, Position: 2, Hash: "hash2", Value: 200},
	}

	go func() {
		<-transport.sent()

		resp := electrum.ListUnspentResp{
			Result: expectedUTXOs,
		}
		jsonResp, err := json.Marshal(map[string]interface{}{"jsonrpc": "2.0", "id": 1, "result": resp.Result})
		require.NoError(t, err)

		transport.responses() <- jsonResp
	}()

	utxos, err := client.ListUnspent(ctx, scripthash)
	require.NoError(t, err)
	if !reflect.DeepEqual(expectedUTXOs, utxos) {
		t.Errorf("Expected UTXOs %v, got %v", expectedUTXOs, utxos)
	}
}
