package sse

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"

	"github.com/splitio/go-toolkit/logging"
)

const (
	// OK It could connect streaming
	OK = iota
	// ErrorOnClientCreation Could not create client
	ErrorOnClientCreation
	// ErrorRequestPerformed Could not perform request
	ErrorRequestPerformed
	// ErrorConnectToStreaming Could not connect to streaming
	ErrorConnectToStreaming
	// ErrorReadingStream Error in streaming
	ErrorReadingStream
)

var sseDelimiter [2]byte = [...]byte{':', ' '}
var sseData [4]byte = [...]byte{'d', 'a', 't', 'a'}
var sseKeepAlive [10]byte = [...]byte{':', 'k', 'e', 'e', 'p', 'a', 'l', 'i', 'v', 'e'}

// SSEClient struct
type SSEClient struct {
	url      string
	client   http.Client
	status   chan int
	stopped  chan struct{}
	shutdown chan struct{}
	logger   logging.LoggerInterface
}

// NewSSEClient creates new SSEClient
func NewSSEClient(url string, status chan int, stopped chan struct{}, logger logging.LoggerInterface) (*SSEClient, error) {
	if cap(status) < 1 {
		return nil, errors.New("Status channel should have length")
	}
	if cap(stopped) < 1 {
		return nil, errors.New("Stopped channel should have length")
	}
	return &SSEClient{
		url:      url,
		client:   http.Client{},
		status:   status,
		stopped:  stopped,
		shutdown: make(chan struct{}, 1),
		logger:   logger,
	}, nil
}

// Shutdown stops SSE
func (l *SSEClient) Shutdown() {
	select {
	case l.shutdown <- struct{}{}:
	default:
		l.logger.Error("Awaited unexpected event")
	}
	<-l.stopped
}

func parseData(raw []byte) (map[string]interface{}, error) {
	data := make(map[string]interface{})
	err := json.Unmarshal(raw, &data)
	if err != nil {
		return nil, fmt.Errorf("error parsing json: %w", err)
	}
	return data, nil
}

func (l *SSEClient) readEvent(reader *bufio.Reader) (map[string]interface{}, error) {
	line, err := reader.ReadBytes('\n')
	if err != nil && err != io.EOF {
		return nil, err
	}

	if len(line) < 2 {
		return nil, nil
	}
	l.logger.Info("LINE:", string(line))

	splitted := bytes.Split(line, sseDelimiter[:])

	if bytes.Contains(splitted[0], sseKeepAlive[:]) {
		data := make(map[string]interface{})
		data["event"] = string(sseKeepAlive[1:])
		return data, nil
	}

	if bytes.Compare(splitted[0], sseData[:]) != 0 {
		return nil, nil
	}

	raw := bytes.TrimSpace(splitted[1])
	data, err := parseData(raw)
	if err != nil {
		l.logger.Error("Error parsing event: ", err)
		return nil, nil
	}

	return data, nil
}

// Do starts streaming
func (l *SSEClient) Do(params map[string]string, callback func(e map[string]interface{})) {
	defer func() { l.stopped <- struct{}{} }()

	req, err := http.NewRequest("GET", l.url, nil)
	if err != nil {
		l.logger.Error(err)
		l.status <- ErrorOnClientCreation
		return
	}

	query := req.URL.Query()

	for key, value := range params {
		query.Add(key, value)
	}
	req.URL.RawQuery = query.Encode()
	req.Header.Set("Accept", "text/event-stream")

	resp, err := l.client.Do(req)
	if err != nil {
		l.logger.Error(err)
		l.status <- ErrorRequestPerformed
		return
	}
	if resp.StatusCode != 200 {
		l.status <- ErrorConnectToStreaming
		return
	}

	l.status <- OK
	reader := bufio.NewReader(resp.Body)
	defer resp.Body.Close()

	shouldKeepRunning := true
	activeGoroutines := sync.WaitGroup{}

	for shouldKeepRunning {
		select {
		case <-l.shutdown:
			l.logger.Info("Shutting down listener")
			shouldKeepRunning = false
		default:
			event, err := l.readEvent(reader)
			if err != nil {
				l.status <- ErrorReadingStream
				l.Shutdown()
				return
			}

			if event != nil {
				activeGoroutines.Add(1)
				go func() {
					defer activeGoroutines.Done()
					callback(event)
				}()
			}
		}
	}
	l.logger.Info("SSE streaming exiting")
	activeGoroutines.Wait()
}
