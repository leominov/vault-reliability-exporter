package main

import (
	"fmt"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/push"
)

const (
	BucketTotal = "total"
	BucketAuth  = "auth"
	BucketRead  = "read"
	BucketWrite = "write"
)

var (
	defaultBucket = map[float64]float64{
		0.005: 0.0,
		0.01:  0.0,
		0.025: 0.0,
		0.05:  0.0,
		0.1:   0.0,
		0.25:  0.0,
		0.5:   0.0,
		1:     0.0,
		2.5:   0.0,
		5:     0.0,
		10:    0.0,
		20:    0.0,
		30:    0.0,
		40:    0.0,
		50:    0.0,
	}
)

type Exporter struct {
	config       *Config
	execTime     *prometheus.GaugeVec
	scrapeTime   *prometheus.GaugeVec
	errors       *prometheus.GaugeVec
	totalScrapes *prometheus.CounterVec
	duration     *prometheus.GaugeVec

	execBucketCounters map[string]map[float64]float64
}

func NewVaultExporter(config *Config) *Exporter {
	return &Exporter{
		config: config,
		execTime: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: config.Namespace,
			Name:      "execution_time_bucket",
			Help:      "Execution time.",
		}, joinWithLabelsMap([]string{"le", "type"}, config.Labels)),
		scrapeTime: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: config.Namespace,
			Name:      "scrape_time",
			Help:      "The last scrape time.",
		}, joinWithLabelsMap([]string{}, config.Labels)),
		totalScrapes: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: config.Namespace,
			Name:      "scrapes_total",
			Help:      "Current total vault scrapes.",
		}, joinWithLabelsMap([]string{}, config.Labels)),
		errors: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: config.Namespace,
			Name:      "errors_total",
			Help:      "Current total errors.",
		}, joinWithLabelsMap([]string{"type"}, config.Labels)),
		duration: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: config.Namespace,
			Name:      "last_scrape_duration_seconds",
			Help:      "The last scrape duration.",
		}, joinWithLabelsMap([]string{}, config.Labels)),
		execBucketCounters: map[string]map[float64]float64{
			BucketTotal: copyMap(defaultBucket),
			BucketAuth:  copyMap(defaultBucket),
			BucketRead:  copyMap(defaultBucket),
			BucketWrite: copyMap(defaultBucket),
		},
	}
}

func (e *Exporter) AddGaugeValues(g *prometheus.GaugeVec, val []string) prometheus.Gauge {
	var values []string
	if len(val) != 0 {
		values = val
	}
	values = append(values, labelValues(e.config.Labels)...)
	return g.WithLabelValues(values...)
}

func (e *Exporter) AddCounterValues(c *prometheus.CounterVec, val []string) prometheus.Counter {
	var values []string
	if len(val) != 0 {
		values = val
	}
	values = append(values, labelValues(e.config.Labels)...)
	return c.WithLabelValues(values...)
}

func (e *Exporter) send() {
	for key, bucket := range e.execBucketCounters {
		for le, val := range bucket {
			e.AddGaugeValues(
				e.execTime,
				[]string{
					fmt.Sprintf("%v", le),
					key,
				}).
				Set(val)
		}
	}
	if err := push.Collectors(
		e.config.JobName,
		push.HostnameGroupingKey(),
		e.config.PushgatewayURL,
		e.scrapeTime,
		e.totalScrapes,
		e.errors,
		e.duration,
		e.execTime,
	); err != nil {
		logrus.Errorf("Could not push to Pushgateway: %v", err)
	}
}

func (e *Exporter) collect() error {
	var (
		now      int64
		duration float64
	)
	// Check aut
	now = time.Now().UnixNano()
	vaultCli, err := NewClient(
		e.config.Addr,
		e.config.AuthLogin,
		e.config.AuthPassword,
		e.config.AuthMethod,
		e.config.ClientTimeout,
	)
	if err != nil {
		e.AddGaugeValues(e.errors, []string{BucketAuth}).Inc()
		logrus.Error(err)
		return err
	}
	duration = float64(time.Now().UnixNano()-now) / 1000000000
	e.IncBucketCounter(BucketAuth, duration)

	// Check write
	now = time.Now().UnixNano()
	_, err = vaultCli.Logical().Write(
		e.config.SecretPath,
		map[string]interface{}{
			"foo": "bar",
		},
	)
	if err != nil {
		e.AddGaugeValues(e.errors, []string{BucketWrite}).Inc()
		logrus.Error(err)
		return err
	}
	duration = float64(time.Now().UnixNano()-now) / 1000000000
	e.IncBucketCounter(BucketWrite, duration)

	// Check read
	now = time.Now().UnixNano()
	_, err = vaultCli.Logical().Read(e.config.SecretPath)
	if err != nil {
		e.AddGaugeValues(e.errors, []string{BucketRead}).Inc()
		logrus.Error(err)
		return err
	}
	duration = float64(time.Now().UnixNano()-now) / 1000000000
	e.IncBucketCounter(BucketRead, duration)

	return nil
}

func (e *Exporter) IncBucketCounter(name string, duration float64) {
	logrus.Debugf("%s = %v", name, duration)
	for d := range e.execBucketCounters[name] {
		if duration >= d {
			logrus.Debugf("Inc %s le=%v", name, d)
			e.execBucketCounters[name][d]++
		}
	}
}

func (e *Exporter) resetErrorCounters() {
	e.AddGaugeValues(e.errors, []string{BucketTotal}).Set(0.0)
	e.AddGaugeValues(e.errors, []string{BucketAuth}).Set(0.0)
	e.AddGaugeValues(e.errors, []string{BucketRead}).Set(0.0)
	e.AddGaugeValues(e.errors, []string{BucketWrite}).Set(0.0)
}

func (e *Exporter) Collect() {
	e.resetErrorCounters()
	for {
		select {
		case <-time.NewTicker(e.config.Interval).C:
			logrus.Debug("Tick")
			e.AddCounterValues(e.totalScrapes, nil).Inc()
			e.AddGaugeValues(e.scrapeTime, nil).SetToCurrentTime()

			now := time.Now().UnixNano()
			if err := e.collect(); err != nil {
				e.AddGaugeValues(e.errors, []string{BucketTotal}).Inc()
			}
			duration := float64(time.Now().UnixNano()-now) / 1000000000

			e.AddGaugeValues(e.duration, nil).Set(duration)
			e.IncBucketCounter(BucketTotal, duration)

			for name, bucker := range e.execBucketCounters {
				logrus.Debugf("Counters %s: %#v", name, bucker)
			}

			e.send()
		}
	}
}
