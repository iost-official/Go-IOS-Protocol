package metrics

import (
	"errors"
	"sync/atomic"
	"time"

	"github.com/iost-official/go-iost/v3/ilog"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/push"
)

// errors
var (
	ErrNilPusher         = errors.New("pusher is nil")
	ErrPusherUnavailable = errors.New("pusher addr unavailable")
)

var (
	pushInterval = time.Millisecond * 500
)

// Client is the struct responsible for sending different type of metrics.
type Client struct {
	isRunning uint32

	pusher *push.Pusher
	exitCh chan struct{}

	collectorCache []prometheus.Collector
}

// NewClient returns a new Client.
func NewClient() *Client {
	return &Client{
		exitCh:         make(chan struct{}),
		collectorCache: make([]prometheus.Collector, 0),
	}
}

// SetPusher sets the pusher with the given addr.
func (c *Client) SetPusher(addr, username, password string) error {
	c.pusher = push.New(addr, "iost")
	c.pusher.BasicAuth(username, password)
	for _, colloctor := range c.collectorCache {
		c.pusher.Collector(colloctor)
	}
	c.pusher.Collector(collectors.NewGoCollector())
	return nil
}

// SetID sets the ID of metrics client.
func (c *Client) SetID(id string) {
	if c.pusher == nil || id == "" {
		return
	}
	c.pusher.Grouping("_id", id)
}

// Start starts the pusher loop.
func (c *Client) Start() error {
	if c.pusher == nil {
		return ErrNilPusher
	}
	if !atomic.CompareAndSwapUint32(&c.isRunning, 0, 1) {
		return nil
	}
	go c.startPush()
	return nil
}

// Stop stops the pusher loop.
func (c *Client) Stop() {
	if c.pusher == nil {
		return
	}
	if !atomic.CompareAndSwapUint32(&c.isRunning, 1, 0) {
		return
	}
	c.exitCh <- struct{}{}
	<-c.exitCh
	c.pusher.Push()
}

// NewCounter returns a counter-type metrics.
func (c *Client) NewCounter(name string, labels []string) Counter {
	counterVec := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: name,
		Help: "-",
	}, labels)
	if c.pusher != nil {
		c.pusher.Collector(counterVec)
	} else {
		c.collectorCache = append(c.collectorCache, counterVec)
	}
	return NewPromCounter(counterVec)
}

// NewGauge returns a gauge-type metrics.
func (c *Client) NewGauge(name string, labels []string) Gauge {
	gaugeVec := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: name,
		Help: "-",
	}, labels)
	if c.pusher != nil {
		c.pusher.Collector(gaugeVec)
	} else {
		c.collectorCache = append(c.collectorCache, gaugeVec)
	}
	return NewPromGauge(gaugeVec)
}

// NewSummary returns a summary-type metrics.
func (c *Client) NewSummary(name string, labels []string) Summary {
	summaryVec := prometheus.NewSummaryVec(prometheus.SummaryOpts{
		Name: name,
		Help: "-",
	}, labels)
	if c.pusher != nil {
		c.pusher.Collector(summaryVec)
	} else {
		c.collectorCache = append(c.collectorCache, summaryVec)
	}
	return NewPromSummary(summaryVec)
}

func (c *Client) startPush() {
	timer := time.NewTimer(pushInterval)
	for {
		select {
		case <-timer.C:
			err := c.pusher.Push()
			if err != nil {
				ilog.Warnf("push metrics failed:%v", err)
			}
			timer.Reset(pushInterval)
		case <-c.exitCh:
			timer.Stop()
			c.exitCh <- struct{}{}
			return
		}
	}
}
