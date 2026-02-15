package electrum_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zauberhaus/go-electrum/electrum"
	"github.com/zauberhaus/logger"
)

func TestSubscribeHeaders(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	log := logger.GetLogger(ctx)
	//log.EnableDebug()
	log = log.AddSkip(-1)
	ctx = logger.AddLogger(ctx, log)

	client, err := electrum.NewClientTCP(context.Background(), "bch.imaginary.cash:50001")
	require.NoError(t, err)

	defer client.Shutdown()

	headerChan, err := client.SubscribeHeaders(ctx)
	require.NoError(t, err)

	// first header from subscribe
	header := <-headerChan

	if assert.NotNil(t, header) {
		assert.NotZero(t, header.Height)
		assert.NotEmpty(t, header.Hex)
	}
}

func TestSubscribeHeaders_Mock(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	log := logger.GetLogger(ctx)
	//log.EnableDebug()
	log = log.AddSkip(-1)
	ctx = logger.AddLogger(ctx, log)

	client, transport := newTestClient(ctx, t)
	defer client.Shutdown()

	expectedHeader := &electrum.SubscribeHeadersResult{
		Height: 123,
		Hex:    "header_hex",
	}

	var headerChan <-chan *electrum.SubscribeHeadersResult

	err := transport.exec(1, expectedHeader, func() error {
		ch, err := client.SubscribeHeaders(ctx)
		headerChan = ch
		return err
	})

	require.NoError(t, err)

	// first header from subscribe
	header := <-headerChan
	assert.Equal(t, expectedHeader, header)

	notif := &electrum.SubscribeHeadersNotif{
		Params: []*electrum.SubscribeHeadersResult{
			{
				Height: 123,
				Hex:    "header_hex",
			},
		},
	}

	transport.notify("blockchain.headers.subscribe", notif.Params)

	header = <-headerChan
	assert.Equal(t, notif.Params[0], header)
}

func TestScripthashSubscription(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client, transport := newTestClient(ctx, t)
	defer client.Shutdown()

	sub, notifChan := client.SubscribeScripthash()

	scripthash := "scripthash"
	address := "address"
	expectedStatus := "status"

	err := transport.exec(1, expectedStatus, func() error {
		return sub.Add(ctx, scripthash, address)
	})
	require.NoError(t, err)

	notif := <-notifChan
	assert.Equal(t, [2]string{scripthash, expectedStatus}, notif.Params)

	// test GetAddress
	addr, err := sub.GetAddress(scripthash)
	require.NoError(t, err)
	assert.Equal(t, address, addr)

	// test GetScripthash
	sh, err := sub.GetScripthash(address)
	require.NoError(t, err)
	assert.Equal(t, scripthash, sh)

	params := [2]string{scripthash, "new_status"}

	err = transport.notify("blockchain.scripthash.subscribe", params)
	require.NoError(t, err)

	notif = <-notifChan
	assert.Equal(t, params, notif.Params)

	// test Remove
	err = transport.exec(2, "status", func() error {
		return sub.Remove(ctx, scripthash)
	})

	require.NoError(t, err)
	assert.Equal(t, 0, len(sub.SH()))

	err = transport.exec(3, "status", func() error {
		return sub.Add(ctx, scripthash, address)
	})
	require.NoError(t, err)

	notif = <-notifChan // consume notification
	err = sub.RemoveAddress(address)
	require.NoError(t, err)
	assert.Equal(t, 0, len(sub.SH()))

	err = transport.exec(4, "status", func() error {
		return sub.Add(ctx, scripthash, address)
	})
	require.NoError(t, err)
	notif = <-notifChan // consume notification

	err = transport.exec(5, "resubscribed", func() error {
		return sub.Resubscribe(ctx)
	})
	require.NoError(t, err)
	notif = <-notifChan // consume notification
	assert.Equal(t, [2]string{scripthash, "resubscribed"}, notif.Params)
}

func TestSubscribeMasternode(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client, transport := newTestClient(ctx, t)
	defer client.Shutdown()

	collateral := "collateral"
	expectedStatus := "status"

	go func() {
		<-transport.sent()
		jsonResp, err := json.Marshal(map[string]interface{}{"jsonrpc": "2.0", "id": 1, "result": expectedStatus})
		require.NoError(t, err)

		transport.responses() <- jsonResp
	}()

	statusChan, err := client.SubscribeMasternode(ctx, collateral)
	require.NoError(t, err)
	time.Sleep(1 * time.Second)

	// first status from subscribe
	status := <-statusChan
	assert.Equal(t, expectedStatus, status)

	// now test a notification
	go func() {
		notif := &electrum.SubscribeNotif{
			Params: [2]string{collateral, "new_status"},
		}
		jsonNotif, err := json.Marshal(map[string]interface{}{"jsonrpc": "2.0", "method": "blockchain.masternode.subscribe", "params": notif.Params})
		require.NoError(t, err)
		transport.responses() <- jsonNotif
	}()

	status = <-statusChan
	assert.Equal(t, collateral, status)

	status = <-statusChan
	assert.Equal(t, "new_status", status)
}
