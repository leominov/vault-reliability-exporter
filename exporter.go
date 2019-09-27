package main

import (
	"net"
	"net/http"
	"os"
	"time"

	"github.com/hashicorp/vault/api"
	"github.com/sirupsen/logrus"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/push"
)

const (
	BucketTotal  = "total"
	BucketAuth   = "auth"
	BucketRead   = "read"
	BucketWrite  = "write"
	BucketRevoke = "revoke"
)

var (
	defaultBuckets = []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10, 20, 30, 40, 50}

	defaultTransport = &http.Transport{
		Dial: (&net.Dialer{
			Timeout: 5 * time.Second,
		}).Dial,
		TLSHandshakeTimeout: 5 * time.Second,
	}
)

type Exporter struct {
	config *Config
	pusher *push.Pusher

	// profile
	errors        *prometheus.GaugeVec
	execHistogram *prometheus.HistogramVec

	// global
	scrapeTime   prometheus.Gauge
	totalScrapes prometheus.Counter
	duration     prometheus.Gauge

	execBucketCounters map[string]map[float64]float64
}

func NewVaultExporter(config *Config) *Exporter {
	e := &Exporter{
		config: config,
		scrapeTime: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: config.PGW.Namespace,
			Name:      "scrape_time",
			Help:      "The last scrape time.",
		}),
		totalScrapes: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: config.PGW.Namespace,
			Name:      "scrapes_total",
			Help:      "Current total vault scrapes.",
		}),
		errors: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: config.PGW.Namespace,
			Name:      "errors_total",
			Help:      "Current total errors.",
		}, []string{"type", "profile"}),
		duration: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: config.PGW.Namespace,
			Name:      "last_scrape_duration_seconds",
			Help:      "The last scrape duration.",
		}),
		execHistogram: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: config.PGW.Namespace,
			Name:      "execution_duration_seconds",
			Help:      "Execution time.",
			Buckets:   defaultBuckets,
		}, []string{"type", "profile"}),
	}
	e.setupPusher()
	return e
}

func (e *Exporter) setupPusher() {
	e.pusher = push.New(e.config.PGW.Addr, e.config.PGW.Job)

	if e.config.PGW.BasicAuth != nil {
		e.pusher.BasicAuth(
			e.config.PGW.BasicAuth.Username,
			e.config.PGW.BasicAuth.Password,
		)
	}

	e.pusher.Client(&http.Client{
		Timeout:   e.config.PGW.Timeout,
		Transport: defaultTransport,
	})

	for k, v := range e.config.PGW.Labels {
		e.pusher.Grouping(k, v)
	}
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}
	e.pusher.Grouping("instance", hostname)

	e.pusher.
		Collector(e.scrapeTime).
		Collector(e.totalScrapes).
		Collector(e.errors).
		Collector(e.duration).
		Collector(e.execHistogram)
}

func (e *Exporter) send() {
	logrus.Debug("Push metrics")

	err := e.pusher.Push()
	if err != nil {
		logrus.Errorf("Could not push to Pushgateway: %v", err)
	}
}

func (e *Exporter) collect(profile *VaultProfile) error {
	log := logrus.WithField("profile", profile.Name)

	// Check auth
	log.Debugf("Login(%s)", profile.AuthPath)
	now := time.Now().UnixNano()
	vaultCli, err := NewClient(
		e.config.Vault.Addr,
		e.config.Vault.Timeout,
		e.config.Vault.MaxRetries,
		profile,
	)
	if err != nil {
		e.errors.WithLabelValues([]string{BucketAuth, profile.Name}...).Inc()
		log.Error(err)
		return err
	}
	duration := float64(time.Now().UnixNano()-now) / 1000000000
	e.execHistogram.WithLabelValues([]string{BucketAuth, profile.Name}...).Observe(duration)

	if len(profile.SecretPath) != 0 {
		// Check read and write
		e.collectVaultReadWrite(vaultCli, profile, log)
	}

	if profile.RevokeToken {
		// Check revoke
		e.collectVaultRevoke(vaultCli, profile, log)
	}

	return nil
}

func (e *Exporter) collectVaultReadWrite(cli *api.Client, profile *VaultProfile, log *logrus.Entry) error {
	log.Debugf("Write(%s, %v)", profile.SecretPath, profile.SecretData)
	now := time.Now().UnixNano()
	_, err := cli.Logical().Write(profile.SecretPath, profile.SecretData)
	if err != nil {
		e.errors.WithLabelValues([]string{BucketWrite, profile.Name}...).Inc()
		log.Error(err)
		return err
	}
	duration := float64(time.Now().UnixNano()-now) / 1000000000
	e.execHistogram.WithLabelValues([]string{BucketWrite, profile.Name}...).Observe(duration)

	log.Debugf("Read(%s)", profile.SecretPath)
	now = time.Now().UnixNano()
	_, err = cli.Logical().Read(profile.SecretPath)
	if err != nil {
		e.errors.WithLabelValues([]string{BucketRead, profile.Name}...).Inc()
		log.Error(err)
		return err
	}
	duration = float64(time.Now().UnixNano()-now) / 1000000000
	e.execHistogram.WithLabelValues([]string{BucketRead, profile.Name}...).Observe(duration)
	return nil
}

func (e *Exporter) collectVaultRevoke(cli *api.Client, profile *VaultProfile, log *logrus.Entry) error {
	log.Debug("Revoke(self)")
	now := time.Now().UnixNano()
	err := cli.Auth().Token().RevokeSelf(cli.Token())
	if err != nil {
		e.errors.WithLabelValues([]string{BucketRevoke, profile.Name}...).Inc()
		log.Error(err)
		return err
	}
	duration := float64(time.Now().UnixNano()-now) / 1000000000
	e.execHistogram.WithLabelValues([]string{BucketRevoke, profile.Name}...).Observe(duration)
	return nil
}

func (e *Exporter) resetErrorCounters() {
	e.errors.WithLabelValues([]string{BucketTotal, "all"}...).Set(0.0)
	for _, profile := range e.config.Vault.Profiles {
		e.errors.WithLabelValues([]string{BucketAuth, profile.Name}...).Set(0.0)
		e.errors.WithLabelValues([]string{BucketRead, profile.Name}...).Set(0.0)
		e.errors.WithLabelValues([]string{BucketWrite, profile.Name}...).Set(0.0)
	}
}

func (e *Exporter) Collect() {
	e.resetErrorCounters()
	for {
		select {
		case <-time.NewTicker(e.config.RepeatInterval).C:
			logrus.Debug("Tick")

			e.totalScrapes.Inc()
			e.scrapeTime.SetToCurrentTime()

			duration := float64(0)
			for _, profile := range e.config.Vault.Profiles {
				now := time.Now().UnixNano()
				if err := e.collect(profile); err != nil {
					e.errors.WithLabelValues([]string{BucketTotal, "all"}...).Inc()
				}
				duration += float64(time.Now().UnixNano()-now) / 1000000000
				time.Sleep(e.config.Delay)
			}

			e.duration.Set(duration)
			e.execHistogram.WithLabelValues([]string{BucketTotal, "all"}...).Observe(duration)

			for name, bucker := range e.execBucketCounters {
				logrus.Debugf("Counters %s: %#v", name, bucker)
			}

			e.send()
		}
	}
}
