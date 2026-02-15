package electrum_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zauberhaus/go-electrum/electrum"
)

func TestGetBlockHeader(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client, transport := newTestClient(ctx, t)
	defer client.Shutdown()

	height := uint32(123)
	expectedHeader := &electrum.GetBlockHeaderResult{
		Header: "block_header",
	}

	go func() {
		<-transport.sent()

		// Create the response for the simple case
		resp := electrum.BasicResp{
			Result: expectedHeader.Header,
		}
		jsonResp, err := json.Marshal(map[string]interface{}{"jsonrpc": "2.0", "id": 1, "result": resp.Result})
		require.NoError(t, err)

		transport.responses() <- jsonResp
	}()

	header, err := client.GetBlockHeader(ctx, height)
	require.NoError(t, err)
	assert.Equal(t, expectedHeader, header)
}

func TestGetBlockHeader_WithCheckpoint(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client, transport := newTestClient(ctx, t)
	defer client.Shutdown()

	height := uint32(123)
	checkpointHeight := uint32(456)
	expectedHeader := &electrum.GetBlockHeaderResult{
		Branch: []string{"branch"},
		Header: "block_header",
		Root:   "root",
	}

	go func() {
		<-transport.sent()

		// Create the response for the checkpoint case
		resp := electrum.GetBlockHeaderResp{
			Result: expectedHeader,
		}
		jsonResp, err := json.Marshal(map[string]interface{}{"jsonrpc": "2.0", "id": 1, "result": resp.Result})
		require.NoError(t, err)

		transport.responses() <- jsonResp
	}()

	header, err := client.GetBlockHeader(ctx, height, checkpointHeight)
	require.NoError(t, err)
	assert.Equal(t, expectedHeader, header)
}

func TestGetBlockHeaders(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client, transport := newTestClient(ctx, t)
	defer client.Shutdown()

	startHeight := uint32(123)
	count := uint32(10)
	expectedHeaders := &electrum.GetBlockHeadersResult{
		Count:   count,
		Headers: "block_headers",
		Max:     count,
	}

	go func() {
		<-transport.sent()

		resp := electrum.GetBlockHeadersResp{
			Result: expectedHeaders,
		}
		jsonResp, err := json.Marshal(map[string]interface{}{"jsonrpc": "2.0", "id": 1, "result": resp.Result})
		require.NoError(t, err)

		transport.responses() <- jsonResp
	}()

	headers, err := client.GetBlockHeaders(ctx, startHeight, count)
	require.NoError(t, err)
	assert.Equal(t, expectedHeaders, headers)
}

func TestGetBlockHeaders_WithCheckpoint(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client, transport := newTestClient(ctx, t)
	defer client.Shutdown()

	startHeight := uint32(123)
	count := uint32(10)
	checkpointHeight := uint32(456)
	expectedHeaders := &electrum.GetBlockHeadersResult{
		Count:   count,
		Headers: "block_headers",
		Max:     count,
		Branch:  []string{"branch"},
		Root:    "root",
	}

	go func() {
		<-transport.sent()

		resp := electrum.GetBlockHeadersResp{
			Result: expectedHeaders,
		}
		jsonResp, err := json.Marshal(map[string]interface{}{"jsonrpc": "2.0", "id": 1, "result": resp.Result})
		require.NoError(t, err)

		transport.responses() <- jsonResp
	}()

	headers, err := client.GetBlockHeaders(ctx, startHeight, count, checkpointHeight)
	require.NoError(t, err)
	assert.Equal(t, expectedHeaders, headers)
}

func TestGetBlockHeader_Error(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client, _ := newTestClient(ctx, t)
	defer client.Shutdown()

	_, err := client.GetBlockHeader(ctx, 200, 100)
	assert.Error(t, err)
	assert.Equal(t, electrum.ErrCheckpointHeight, err)
}

func TestGetBlockHeaders_Error(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client, _ := newTestClient(ctx, t)
	defer client.Shutdown()

	_, err := client.GetBlockHeaders(ctx, 100, 10, 105)
	assert.Error(t, err)
	assert.Equal(t, electrum.ErrCheckpointHeight, err)
}
