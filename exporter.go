package main

import (
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
	defaultBuckets = []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10, 20, 30, 40, 50}
)

type Exporter struct {
	config *Config

	// profile
	errors        *prometheus.GaugeVec
	execHistogram *prometheus.HistogramVec

	// global
	scrapeTime   *prometheus.GaugeVec
	totalScrapes *prometheus.CounterVec
	duration     *prometheus.GaugeVec

	execBucketCounters map[string]map[float64]float64
}

func NewVaultExporter(config *Config) *Exporter {
	e := &Exporter{
		config: config,
		scrapeTime: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: config.PGW.Namespace,
			Name:      "scrape_time",
			Help:      "The last scrape time.",
		}, joinWithLabelsMap([]string{}, config.PGW.Labels)),
		totalScrapes: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: config.PGW.Namespace,
			Name:      "scrapes_total",
			Help:      "Current total vault scrapes.",
		}, joinWithLabelsMap([]string{}, config.PGW.Labels)),
		errors: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: config.PGW.Namespace,
			Name:      "errors_total",
			Help:      "Current total errors.",
		}, joinWithLabelsMap([]string{"type", "profile"}, config.PGW.Labels)),
		duration: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: config.PGW.Namespace,
			Name:      "last_scrape_duration_seconds",
			Help:      "The last scrape duration.",
		}, joinWithLabelsMap([]string{}, config.PGW.Labels)),
		execHistogram: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: config.PGW.Namespace,
			Name:      "execution_time",
			Help:      "Execution time.",
			Buckets:   defaultBuckets,
		}, joinWithLabelsMap([]string{"type", "profile"}, config.PGW.Labels)),
	}
	return e
}

func (e *Exporter) AddGaugeValues(g *prometheus.GaugeVec, val []string) prometheus.Gauge {
	var values []string
	if len(val) != 0 {
		values = val
	}
	values = append(values, labelValues(e.config.PGW.Labels)...)
	return g.WithLabelValues(values...)
}

func (e *Exporter) AddCounterValues(c *prometheus.CounterVec, val []string) prometheus.Counter {
	var values []string
	if len(val) != 0 {
		values = val
	}
	values = append(values, labelValues(e.config.PGW.Labels)...)
	return c.WithLabelValues(values...)
}

func (e *Exporter) AddHistogramValues(h *prometheus.HistogramVec, val []string) prometheus.Observer {
	var values []string
	if len(val) != 0 {
		values = val
	}
	values = append(values, labelValues(e.config.PGW.Labels)...)
	return h.WithLabelValues(values...)
}

func (e *Exporter) send() {
	if err := push.Collectors(
		e.config.PGW.Job,
		push.HostnameGroupingKey(),
		e.config.PGW.Addr,
		e.scrapeTime,
		e.totalScrapes,
		e.errors,
		e.duration,
		e.execHistogram,
	); err != nil {
		logrus.Errorf("Could not push to Pushgateway: %v", err)
	}
}

func (e *Exporter) collect(profile *VaultProfile) error {
	var (
		now      int64
		duration float64
	)

	// Check aut
	now = time.Now().UnixNano()
	vaultCli, err := NewClient(e.config.Vault.Addr, e.config.Vault.Timeout, profile)
	if err != nil {
		e.AddGaugeValues(e.errors, []string{BucketAuth, profile.Name}).Inc()
		logrus.Error(err)
		return err
	}
	duration = float64(time.Now().UnixNano()-now) / 1000000000
	e.AddHistogramValues(e.execHistogram, []string{BucketAuth, profile.Name}).Observe(duration)

	// Check write
	now = time.Now().UnixNano()
	_, err = vaultCli.Logical().Write(profile.SecretPath, profile.SecretData)
	if err != nil {
		e.AddGaugeValues(e.errors, []string{BucketWrite, profile.Name}).Inc()
		logrus.Error(err)
		return err
	}
	duration = float64(time.Now().UnixNano()-now) / 1000000000
	e.AddHistogramValues(e.execHistogram, []string{BucketWrite, profile.Name}).Observe(duration)

	// Check read
	now = time.Now().UnixNano()
	_, err = vaultCli.Logical().Read(profile.SecretPath)
	if err != nil {
		e.AddGaugeValues(e.errors, []string{BucketRead, profile.Name}).Inc()
		logrus.Error(err)
		return err
	}
	duration = float64(time.Now().UnixNano()-now) / 1000000000
	e.AddHistogramValues(e.execHistogram, []string{BucketRead, profile.Name}).Observe(duration)

	return nil
}

func (e *Exporter) resetErrorCounters() {
	e.AddGaugeValues(e.errors, []string{BucketTotal, "all"}).Set(0.0)
	for _, profile := range e.config.Vault.Profiles {
		e.AddGaugeValues(e.errors, []string{BucketAuth, profile.Name}).Set(0.0)
		e.AddGaugeValues(e.errors, []string{BucketRead, profile.Name}).Set(0.0)
		e.AddGaugeValues(e.errors, []string{BucketWrite, profile.Name}).Set(0.0)
	}
}

func (e *Exporter) Collect() {
	e.resetErrorCounters()
	for {
		select {
		case <-time.NewTicker(e.config.RepeatInterval).C:
			logrus.Debug("Tick")

			e.AddCounterValues(e.totalScrapes, nil).Inc()
			e.AddGaugeValues(e.scrapeTime, nil).SetToCurrentTime()

			duration := float64(0)
			for _, profile := range e.config.Vault.Profiles {
				now := time.Now().UnixNano()
				if err := e.collect(profile); err != nil {
					e.AddGaugeValues(e.errors, []string{BucketTotal, "all"}).Inc()
				}
				duration += float64(time.Now().UnixNano()-now) / 1000000000
				time.Sleep(e.config.Delay)
			}

			e.AddGaugeValues(e.duration, nil).Set(duration)
			e.AddHistogramValues(e.execHistogram, []string{BucketTotal, "all"}).Observe(duration)

			for name, bucker := range e.execBucketCounters {
				logrus.Debugf("Counters %s: %#v", name, bucker)
			}

			e.send()
		}
	}
}
