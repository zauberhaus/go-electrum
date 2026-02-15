package electrum_test

import (
	"context"
	"encoding/json"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zauberhaus/go-electrum/electrum"
)

type MockTransport struct {
	a *electrum.Atomic[electrum.Transport]
}

func NewMockTransport() *MockTransport {
	return &MockTransport{
		a: electrum.MakeAtomic[electrum.Transport](&mockTransport{
			responses: make(chan []byte, 10),
			errors:    make(chan error, 10),
			sent:      make(chan []byte, 10),
		}),
	}
}

func (m *MockTransport) Close() error {
	return m.a.Do(func(val electrum.Transport) error {
		return val.Close()
	})
}

func (m *MockTransport) Errors() <-chan error {
	r, _ := electrum.Get(m.a, func(val electrum.Transport) (<-chan error, bool) {
		return val.Errors(), true
	})

	return r
}

func (m *MockTransport) Responses() <-chan []byte {
	return m.responses()
}

func (m *MockTransport) notify(method string, params any) error {
	jsonNotif, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "method": method, "params": params})
	if err != nil {
		return err
	}

	c := m.responses()

	c <- jsonNotif
	return nil
}

func (m *MockTransport) exec(id int, result any, f func() error) error {
	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		err := m.cmd(id, result)
		if err != nil {
			panic(err)
		}

		wg.Done()
	}()

	err := f()

	wg.Wait()

	return err
}

func (m *MockTransport) cmd(id int, result any) error {
	<-m.sent()
	jsonResp, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": id, "result": result})
	if err != nil {
		return err
	}

	m.responses() <- jsonResp
	return nil
}

func (m *MockTransport) responses() chan []byte {
	r, _ := electrum.Get(m.a, func(val electrum.Transport) (chan []byte, bool) {
		if t, ok := val.(*mockTransport); ok {
			return t.responses, true
		}

		return nil, false
	})

	return r
}

func (m *MockTransport) sent() chan []byte {
	r, _ := electrum.Get(m.a, func(val electrum.Transport) (chan []byte, bool) {
		if t, ok := val.(*mockTransport); ok {
			return t.sent, true
		}

		return nil, false
	})

	return r
}

func (m *MockTransport) SendMessage(msg []byte) error {
	return m.a.Do(func(val electrum.Transport) error {
		return val.SendMessage(msg)
	})
}

var _ electrum.Transport = (*MockTransport)(nil)

// mockTransport is a mock implementation of the Transport interface for testing.
type mockTransport struct {
	responses chan []byte
	errors    chan error
	sent      chan []byte
}

/*
func newMockTransport() *Atomic[Transport] {
	return MakeAtomic[Transport](&mockTransport{
		responses: make(chan []byte, 1),
		errors:    make(chan error, 1),
		sent:      make(chan []byte, 1),
	})
}
*/

func (t *mockTransport) SendMessage(message []byte) error {
	t.sent <- message
	return nil
}

func (t *mockTransport) Responses() <-chan []byte {
	return t.responses
}

func (t *mockTransport) Errors() <-chan error {
	return t.errors
}

func (t *mockTransport) Close() error {
	return nil
}

// newTestClient creates a new Client with a mockTransport for testing.
func newTestClient(ctx context.Context, t *testing.T) (*electrum.Client, *MockTransport) {
	transport := NewMockTransport()
	client := electrum.NewClient(ctx, transport)

	// Wait for the listener to start
	time.Sleep(100 * time.Millisecond)

	return client, transport
}

func TestGetFee(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client, transport := newTestClient(ctx, t)
	defer client.Shutdown()

	target := uint32(6)
	expectedFee := float32(0.000123)

	go func() {
		// Wait for the request to be sent
		<-transport.sent()

		// Create the response
		resp := electrum.GetFeeResp{
			Result: expectedFee,
		}
		jsonResp, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "result": resp.Result})
		require.NoError(t, err)

		// Send the response
		transport.responses() <- jsonResp
	}()

	fee, err := client.GetFee(ctx, target)
	require.NoError(t, err)
	assert.Equal(t, expectedFee, fee)
}

func TestGetRelayFee(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client, transport := newTestClient(ctx, t)
	defer client.Shutdown()

	expectedFee := float32(0.00001)

	go func() {
		// Wait for the request to be sent
		<-transport.sent()

		// Create the response
		resp := electrum.GetFeeResp{
			Result: expectedFee,
		}
		jsonResp, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "result": resp.Result})
		require.NoError(t, err)

		// Send the response
		transport.responses() <- jsonResp
	}()

	fee, err := client.GetRelayFee(ctx)
	require.NoError(t, err)
	assert.Equal(t, expectedFee, fee)
}

func TestGetFeeHistogram(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client, transport := newTestClient(ctx, t)
	defer client.Shutdown()

	expectedHistogram := map[uint32]uint64{
		123: 456,
		789: 101,
	}

	go func() {
		// Wait for the request to be sent
		<-transport.sent()

		// Create the response
		resp := electrum.GetFeeHistogramResp{
			Result: [][2]uint64{{123, 456}, {789, 101}},
		}
		jsonResp, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "result": resp.Result})
		require.NoError(t, err)

		// Send the response
		transport.responses() <- jsonResp
	}()

	histogram, err := client.GetFeeHistogram(ctx)
	require.NoError(t, err)
	if !reflect.DeepEqual(expectedHistogram, histogram) {
		t.Errorf("Expected histogram %v, got %v", expectedHistogram, histogram)
	}
}
