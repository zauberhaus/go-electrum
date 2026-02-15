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

func TestBroadcastTransaction(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client, transport := newTestClient(ctx, t)
	defer client.Shutdown()

	rawTx := "raw_tx"
	expectedTxHash := "tx_hash"

	go func() {
		<-transport.sent()
		jsonResp, err := json.Marshal(map[string]interface{}{"jsonrpc": "2.0", "id": 1, "result": expectedTxHash})
		require.NoError(t, err)

		transport.responses() <- jsonResp
	}()

	txHash, err := client.BroadcastTransaction(ctx, rawTx)
	require.NoError(t, err)
	assert.Equal(t, expectedTxHash, txHash)
}

func TestGetTransaction(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client, transport := newTestClient(ctx, t)
	defer client.Shutdown()

	txHash := "tx_hash"
	expectedTx := &electrum.GetTransactionResult{
		Blockhash: "block_hash",
	}

	go func() {
		<-transport.sent()
		jsonResp, err := json.Marshal(map[string]interface{}{"jsonrpc": "2.0", "id": 1, "result": expectedTx})
		require.NoError(t, err)

		transport.responses() <- jsonResp
	}()

	tx, err := client.GetTransaction(ctx, txHash)
	require.NoError(t, err)
	assert.Equal(t, expectedTx, tx)
}

func TestGetRawTransaction(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client, transport := newTestClient(ctx, t)
	defer client.Shutdown()

	txHash := "tx_hash"
	expectedRawTx := "raw_tx"

	go func() {
		<-transport.sent()
		jsonResp, err := json.Marshal(map[string]interface{}{"jsonrpc": "2.0", "id": 1, "result": expectedRawTx})
		require.NoError(t, err)

		transport.responses() <- jsonResp
	}()

	rawTx, err := client.GetRawTransaction(ctx, txHash)
	require.NoError(t, err)
	assert.Equal(t, expectedRawTx, rawTx)
}

func TestGetMerkleProof(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client, transport := newTestClient(ctx, t)
	defer client.Shutdown()

	txHash := "tx_hash"
	height := uint32(123)
	expectedMerkle := &electrum.GetMerkleProofResult{
		Merkle: []string{"merkle1"},
		Height: height,
	}

	go func() {
		<-transport.sent()
		jsonResp, err := json.Marshal(map[string]interface{}{"jsonrpc": "2.0", "id": 1, "result": expectedMerkle})
		require.NoError(t, err)

		transport.responses() <- jsonResp
	}()

	merkle, err := client.GetMerkleProof(ctx, txHash, height)
	require.NoError(t, err)
	assert.Equal(t, expectedMerkle, merkle)
}

func TestGetHashFromPosition(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client, transport := newTestClient(ctx, t)
	defer client.Shutdown()

	height := uint32(123)
	position := uint32(456)
	expectedHash := "tx_hash"

	go func() {
		<-transport.sent()
		jsonResp, err := json.Marshal(map[string]interface{}{"jsonrpc": "2.0", "id": 1, "result": expectedHash})
		require.NoError(t, err)

		transport.responses() <- jsonResp
	}()

	hash, err := client.GetHashFromPosition(ctx, height, position)
	require.NoError(t, err)
	assert.Equal(t, expectedHash, hash)
}

func TestGetMerkleProofFromPosition(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client, transport := newTestClient(ctx, t)
	defer client.Shutdown()

	height := uint32(123)
	position := uint32(456)
	expectedMerkle := &electrum.GetMerkleProofFromPosResult{
		Hash:   "tx_hash",
		Merkle: []string{"merkle1"},
	}

	go func() {
		<-transport.sent()
		jsonResp, err := json.Marshal(map[string]interface{}{"jsonrpc": "2.0", "id": 1, "result": expectedMerkle})
		require.NoError(t, err)

		transport.responses() <- jsonResp
	}()

	merkle, err := client.GetMerkleProofFromPosition(ctx, height, position)
	require.NoError(t, err)
	if !reflect.DeepEqual(expectedMerkle, merkle) {
		t.Errorf("Expected merkle %v, got %v", expectedMerkle, merkle)
	}
}
