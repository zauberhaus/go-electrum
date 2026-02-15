//go:generate go run go.uber.org/mock/mockgen@latest -typed -destination=network_mock.go -package=electrum -source=./network.go

package electrum

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/zauberhaus/logger"
)

const (
	// ClientVersion identifies the client version/name to the remote server
	ClientVersion = "go-electrum1.1"

	// ProtocolVersion identifies the support protocol version to the remote server
	ProtocolVersion = "1.4"

	nl = byte('\n')
)

var (
	// ErrServerConnected throws an error if remote server is already connected.
	ErrServerConnected = errors.New("server is already connected")

	// ErrServerShutdown throws an error if remote server has shutdown.
	ErrServerShutdown = errors.New("server has shutdown")

	// ErrTimeout throws an error if request has timed out
	ErrTimeout = errors.New("request timeout")

	// ErrNotImplemented throws an error if this RPC call has not been implemented yet.
	ErrNotImplemented = errors.New("RPC call is not implemented")

	// ErrDeprecated throws an error if this RPC call is deprecated.
	ErrDeprecated = errors.New("RPC call has been deprecated")
)

// Transport provides interface to server transport.
type Transport interface {
	SendMessage([]byte) error
	Responses() <-chan []byte
	Errors() <-chan error
	Close() error
}

type container struct {
	content []byte
	err     error
}

// Client stores information about the remote server.
type Client struct {
	transport    *Atomic[Transport]
	handlers     *Atomic[map[uint64]chan *container]
	pushHandlers *Atomic[map[string][]chan *container]

	errorChan chan error
	quit      chan struct{}

	nextID uint64

	log logger.Logger
}

func NewClient(ctx context.Context, transport Transport) *Client {
	log := logger.GetLogger(ctx)

	c := &Client{
		handlers:     MakeAtomic(make(map[uint64]chan *container)),
		pushHandlers: MakeAtomic(make(map[string][]chan *container)),

		errorChan: make(chan error),
		quit:      make(chan struct{}),
		log:       log,
	}

	c.transport = MakeAtomic[Transport](transport)
	go c.listen()

	return c
}

// NewClientTCP initialize a new client for remote server and connects to the remote server using TCP
func NewClientTCP(ctx context.Context, addr string) (*Client, error) {
	transport, err := NewTCPTransport(ctx, addr)
	if err != nil {
		return nil, err
	}

	return NewClient(ctx, transport), nil
}

// NewClientSSL initialize a new client for remote server and connects to the remote server using SSL
func NewClientSSL(ctx context.Context, addr string, config *tls.Config) (*Client, error) {
	transport, err := NewSSLTransport(ctx, addr, config)
	if err != nil {
		return nil, err
	}

	return NewClient(ctx, transport), nil
}

type apiErr struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *apiErr) Error() string {
	return fmt.Sprintf("errNo: %d, errMsg: %s", e.Code, e.Message)
}

type APIError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (A APIError) Error() string {
	return fmt.Sprintf("code: %d, error: %s", A.Code, A.Message)
}

type response struct {
	ID     uint64    `json:"id"`
	Method string    `json:"method"`
	Error  *APIError `json:"error,omitempty"`
}

func (s *Client) listen() {
	errors := func() <-chan error {
		ch, _ := s.transport.Get(func(val Transport) (any, bool) {
			return val.Errors(), true
		})

		if ch == nil {
			return nil
		}

		if ch, ok := ch.(<-chan error); ok {
			return ch
		}

		return nil
	}

	responses := func() <-chan []byte {
		ch, _ := s.transport.Get(func(val Transport) (any, bool) {
			return val.Responses(), true
		})

		if ch == nil {
			return nil
		}

		if ch, ok := ch.(<-chan []byte); ok {
			return ch
		}

		return nil
	}

	for {
		if s.IsShutdown() {
			break
		}
		if s.transport.IsNilOrZero() {
			break
		}
		select {
		case <-s.quit:
			return
		case err := <-errors():
			s.errorChan <- err
			s.Shutdown()
		case bytes := <-responses():
			result := &container{
				content: bytes,
			}

			msg := &response{}
			err := json.Unmarshal(bytes, msg)
			if err != nil {
				s.log.Errorf("Unmarshal received message failed: %v", err)
				result.err = fmt.Errorf("unmarshal received message failed: %v", err)
			} else if msg.Error != nil {
				result.err = msg.Error
			}

			if len(msg.Method) > 0 {
				handlers, ok := Get(s.pushHandlers, func(val map[string][]chan *container) ([]chan *container, bool) {
					handlers, ok := val[msg.Method]
					return handlers, ok
				})

				if ok {
					for _, handler := range handlers {
						handler <- result
					}
				} else {
					s.log.Warnf("Unknown notification: %s -> %s", msg.Method, result.content)
				}
			} else {
				c, ok := Get(s.handlers, func(val map[uint64]chan *container) (chan *container, bool) {
					c, ok := val[msg.ID]

					if c == nil {
						return nil, false
					}

					return c, ok
				})

				if ok {
					c <- result
				} else {
					s.log.Warnf("Unexpected container: %v -> %s", msg.ID, result.content)
				}
			}
		}
	}
}

func (s *Client) listenPush(method string) <-chan *container {
	c := make(chan *container, 1)
	s.pushHandlers.Change(func(val map[string][]chan *container) (map[string][]chan *container, error) {
		val[method] = append(val[method], c)
		return val, nil
	})

	return c
}

type request struct {
	ID     uint64 `json:"id"`
	Method string `json:"method"`
	Params []any  `json:"params"`
}

func (s *Client) request(ctx context.Context, method string, params []any, v any) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	select {
	case <-s.quit:
		return ErrServerShutdown
	default:
	}

	msg := request{
		ID:     atomic.AddUint64(&s.nextID, 1),
		Method: method,
		Params: params,
	}

	bytes, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	bytes = append(bytes, nl)

	err = s.transport.Do(func(val Transport) error {
		return val.SendMessage(bytes)
	})

	if err != nil {
		s.Shutdown()
		return err
	}

	c := make(chan *container, 1)

	err = s.handlers.Change(func(val map[uint64]chan *container) (map[uint64]chan *container, error) {
		if s.IsShutdown() {
			return val, ErrServerShutdown
		}

		if val == nil {
			val = make(map[uint64]chan *container)
		}

		val[msg.ID] = c
		return val, nil
	})

	if err != nil {
		return err
	}

	defer func() {
		s.handlers.Change(func(val map[uint64]chan *container) (map[uint64]chan *container, error) {
			delete(val, msg.ID)
			return nil, nil
		})
	}()

	var resp *container
	select {
	case resp = <-c:
	case <-ctx.Done():
		return ctx.Err()
	}

	if resp.err != nil {
		return resp.err
	}

	if v != nil {
		err = json.Unmarshal(resp.content, v)
		if err != nil {
			return err
		}
	}

	return nil
}

func (s *Client) Shutdown() {

	close(s.quit)

	s.transport.Do(func(val Transport) error {
		if val != nil {
			val.Close()
		}

		return nil
	})

	s.transport.Reset()
	s.handlers.Reset()
	s.pushHandlers.Reset()
}

func (s *Client) IsShutdown() bool {
	select {
	case <-s.quit:
		return true
	default:
	}
	return false
}
