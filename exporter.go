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
	scrapeTime   prometheus.Gauge
	authErrors   prometheus.Gauge
	readErrors   prometheus.Gauge
	writeErrors  prometheus.Gauge
	totalScrapes prometheus.Counter
	duration     prometheus.Gauge

	execBucketCounters map[string]map[float64]float64
}

func copyMap(originalMap map[float64]float64) map[float64]float64 {
	newMap := make(map[float64]float64)
	for key, value := range originalMap {
		newMap[key] = value
	}
	return newMap
}

func NewVaultExporter(config *Config) *Exporter {
	return &Exporter{
		config: config,
		execTime: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: config.Namespace,
			Name:      "execution_time_bucket",
			Help:      "Execution time.",
		}, []string{
			"le", "type",
		}),
		scrapeTime: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: config.Namespace,
			Name:      "exporter_scrape_time",
			Help:      "The last scrape time.",
		}),
		totalScrapes: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: config.Namespace,
			Name:      "exporter_scrapes_total",
			Help:      "Current total redis scrapes.",
		}),
		authErrors: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: config.Namespace,
			Name:      "exporter_auth_error_total",
			Help:      "The last auth error status.",
		}),
		readErrors: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: config.Namespace,
			Name:      "exporter_read_error_total",
			Help:      "The last read error status.",
		}),
		writeErrors: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: config.Namespace,
			Name:      "exporter_write_error_total",
			Help:      "The last write error status.",
		}),
		duration: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: config.Namespace,
			Name:      "exporter_last_scrape_duration_seconds",
			Help:      "The last scrape duration.",
		}),
		execBucketCounters: map[string]map[float64]float64{
			BucketTotal: copyMap(defaultBucket),
			BucketAuth:  copyMap(defaultBucket),
			BucketRead:  copyMap(defaultBucket),
			BucketWrite: copyMap(defaultBucket),
		},
	}
}

func (e *Exporter) send() {
	for key, bucket := range e.execBucketCounters {
		for le, val := range bucket {
			e.execTime.WithLabelValues(
				fmt.Sprintf("%v", le),
				key,
			).Set(val)
		}
	}
	if err := push.Collectors(
		e.config.JobName,
		push.HostnameGroupingKey(),
		e.config.PushgatewayURL,
		e.scrapeTime,
		e.totalScrapes,
		e.authErrors,
		e.readErrors,
		e.writeErrors,
		e.duration,
		e.execTime,
	); err != nil {
		logrus.Errorf("Could not push to Pushgateway: %v", err)
	}
}

func (e *Exporter) collect() {
	var (
		now      int64
		duration float64
	)
	// Check aut
	now = time.Now().UnixNano()
	vaultCli, err := NewClient(e.config.Addr, e.config.AuthLogin, e.config.AuthPassword, e.config.AuthMethod)
	if err != nil {
		e.authErrors.Inc()
		logrus.Error(err)
		return
	}
	duration = float64(time.Now().UnixNano()-now) / 1000000000
	e.IncBucketCounter(BucketAuth, duration)

	// Check write
	now = time.Now().UnixNano()
	_, err = vaultCli.Logical().Write(e.config.SecretPath, map[string]interface{}{
		"foo": "bar",
	})
	if err != nil {
		e.writeErrors.Inc()
		logrus.Error(err)
		return
	}
	duration = float64(time.Now().UnixNano()-now) / 1000000000
	e.IncBucketCounter(BucketWrite, duration)

	// Check read
	now = time.Now().UnixNano()
	_, err = vaultCli.Logical().Read(e.config.SecretPath)
	if err != nil {
		e.readErrors.Inc()
		logrus.Error(err)
		return
	}
	duration = float64(time.Now().UnixNano()-now) / 1000000000
	e.IncBucketCounter(BucketRead, duration)
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

func (e *Exporter) Collect() {
	for {
		select {
		case <-time.NewTicker(e.config.Interval).C:
			logrus.Debug("Tick")
			e.totalScrapes.Inc()
			e.scrapeTime.SetToCurrentTime()

			now := time.Now().UnixNano()
			e.collect()
			duration := float64(time.Now().UnixNano()-now) / 1000000000

			e.duration.Set(duration)
			e.IncBucketCounter(BucketTotal, duration)

			for name, bucker := range e.execBucketCounters {
				logrus.Debugf("Counters %s: %#v", name, bucker)
			}

			e.send()
		}
	}
}
