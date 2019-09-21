package main

import (
	"fmt"
	"strings"
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
	config *Config

	// profile
	execTime *prometheus.GaugeVec
	errors   *prometheus.GaugeVec

	// global
	scrapeTime   *prometheus.GaugeVec
	totalScrapes *prometheus.CounterVec
	duration     *prometheus.GaugeVec

	execBucketCounters map[string]map[float64]float64
}

func NewVaultExporter(config *Config) *Exporter {
	e := &Exporter{
		config: config,
		execTime: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: config.PGW.Namespace,
			Name:      "execution_time_bucket",
			Help:      "Execution time.",
		}, joinWithLabelsMap([]string{"le", "type", "profile"}, config.PGW.Labels)),
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
		execBucketCounters: make(map[string]map[float64]float64),
	}

	e.execBucketCounters[fmt.Sprintf("%s/%s", BucketTotal, "all")] = copyMap(defaultBucket)
	for _, profile := range config.Vault.Profiles {
		e.execBucketCounters[fmt.Sprintf("%s/%s", BucketAuth, profile.Name)] = copyMap(defaultBucket)
		e.execBucketCounters[fmt.Sprintf("%s/%s", BucketRead, profile.Name)] = copyMap(defaultBucket)
		e.execBucketCounters[fmt.Sprintf("%s/%s", BucketWrite, profile.Name)] = copyMap(defaultBucket)
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

func (e *Exporter) send() {
	for key, bucket := range e.execBucketCounters {
		data := strings.SplitN(key, "/", 2)
		if len(data) != 2 {
			continue
		}
		for le, val := range bucket {
			vals := []string{fmt.Sprintf("%v", le), data[0], data[1]}
			e.AddGaugeValues(e.execTime, vals).Set(val)
		}
	}
	if err := push.Collectors(
		e.config.PGW.Job,
		push.HostnameGroupingKey(),
		e.config.PGW.Addr,
		e.scrapeTime,
		e.totalScrapes,
		e.errors,
		e.duration,
		e.execTime,
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
	e.IncBucketCounter(BucketAuth, profile.Name, duration)

	// Check write
	now = time.Now().UnixNano()
	_, err = vaultCli.Logical().Write(profile.SecretPath, profile.SecretData)
	if err != nil {
		e.AddGaugeValues(e.errors, []string{BucketWrite, profile.Name}).Inc()
		logrus.Error(err)
		return err
	}
	duration = float64(time.Now().UnixNano()-now) / 1000000000
	e.IncBucketCounter(BucketWrite, profile.Name, duration)

	// Check read
	now = time.Now().UnixNano()
	_, err = vaultCli.Logical().Read(profile.SecretPath)
	if err != nil {
		e.AddGaugeValues(e.errors, []string{BucketRead, profile.Name}).Inc()
		logrus.Error(err)
		return err
	}
	duration = float64(time.Now().UnixNano()-now) / 1000000000
	e.IncBucketCounter(BucketRead, profile.Name, duration)

	return nil
}

func (e *Exporter) IncBucketCounter(name, profile string, duration float64) {
	nameWithProfile := fmt.Sprintf("%s/%s", name, profile)
	logrus.Debugf("%s = %v", nameWithProfile, duration)
	for d := range e.execBucketCounters[nameWithProfile] {
		if duration >= d {
			logrus.Debugf("Inc %s le=%v", nameWithProfile, d)
			e.execBucketCounters[nameWithProfile][d]++
		}
	}
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
			e.IncBucketCounter(BucketTotal, "all", duration)

			for name, bucker := range e.execBucketCounters {
				logrus.Debugf("Counters %s: %#v", name, bucker)
			}

			e.send()
		}
	}
}
