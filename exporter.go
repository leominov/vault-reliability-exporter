package main

import (
	"time"

	"github.com/sirupsen/logrus"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/push"
)

type Exporter struct {
	config       *Config
	scrapeTime   prometheus.Gauge
	authErrors   prometheus.Gauge
	readErrors   prometheus.Gauge
	writeErrors  prometheus.Gauge
	totalScrapes prometheus.Counter
	duration     prometheus.Gauge
}

func NewVaultExporter(config *Config) *Exporter {
	return &Exporter{
		config: config,
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
	}
}

func (e *Exporter) send() {
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
	); err != nil {
		logrus.Errorf("Could not push to Pushgateway: %v", err)
	}
}

func (e *Exporter) collect() {
	// Check aut
	vaultCli, err := NewClient(e.config.Addr, e.config.AuthLogin, e.config.AuthPassword, e.config.AuthMethod)
	if err != nil {
		e.authErrors.Inc()
		logrus.Error(err)
		return
	}

	// Check write
	_, err = vaultCli.Logical().Write(e.config.SecretPath, map[string]interface{}{
		"foo": "bar",
	})
	if err != nil {
		e.writeErrors.Inc()
		logrus.Error(err)
		return
	}

	// Check read
	_, err = vaultCli.Logical().Read(e.config.SecretPath)
	if err != nil {
		e.readErrors.Inc()
		logrus.Error(err)
		return
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
			e.duration.Set(float64(time.Now().UnixNano()-now) / 1000000000)

			e.send()
		}
	}
}
