package main

import (
	"time"

	"github.com/sirupsen/logrus"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/push"
)

const secretKeyPath = "probe-secrets/test"

type Exporter struct {
	config       *Config
	scrapeTime   prometheus.Gauge
	authErrors   prometheus.Gauge
	readErrors   prometheus.Gauge
	writeErrors  prometheus.Gauge
	totalScrapes prometheus.Counter
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
			Name:      "exporter_last_auth_error",
			Help:      "The last auth error status.",
		}),
		readErrors: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: config.Namespace,
			Name:      "exporter_last_read_error",
			Help:      "The last read error status.",
		}),
		writeErrors: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: config.Namespace,
			Name:      "exporter_last_write_error",
			Help:      "The last write error status.",
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
	); err != nil {
		logrus.Errorf("Could not push to Pushgateway: %v", err)
	}
}

func (e *Exporter) collect() {
	vaultCli, err := NewClient(e.config.Addr, e.config.AuthLogin, e.config.AuthPassword, e.config.AuthMethod)
	if err != nil {
		e.authErrors.Set(1.0)
		logrus.Error(err)
		return
	} else {
		e.authErrors.Set(0.0)
	}
	_, err = vaultCli.Logical().Write(secretKeyPath, map[string]interface{}{
		"foo": "bar",
	})
	if err != nil {
		e.writeErrors.Set(1.0)
		logrus.Error(err)
		return
	} else {
		e.writeErrors.Set(0.0)
	}
	_, err = vaultCli.Logical().Read(secretKeyPath)
	if err != nil {
		e.readErrors.Set(1.0)
		logrus.Error(err)
		return
	} else {
		e.readErrors.Set(0.0)
	}
}

func (e *Exporter) Collect() {
	for {
		select {
		case <-time.NewTicker(e.config.Interval).C:
			logrus.Debug("Tick")
			e.totalScrapes.Inc()
			e.scrapeTime.SetToCurrentTime()
			e.collect()
			e.send()
		}
	}
}
