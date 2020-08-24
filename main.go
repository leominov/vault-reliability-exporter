package main

import (
	"flag"
	"fmt"
	"net/http"
	_ "net/http/pprof"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/version"
	"github.com/sirupsen/logrus"
)

const (
	name = "Vault Reliability Exporter"
)

var (
	flagVersion       = flag.Bool("version", false, "Prints version and exit.")
	flagLogFormat     = flag.String("log-format", "txt", "Log format, valid options are txt and json.")
	flagDebug         = flag.Bool("debug", false, "Output verbose debug information.")
	flagConfigFile    = flag.String("config", "/etc/vault-reliability-exporter/config.yaml", "Path to configuration file.")
	flagCheck         = flag.Bool("check", false, "Check configuration and exit.")
	flagListenAddress = flag.String("web.listen-address", ":9356", "Address to listen on for telemetry.")
	flagMetricPath    = flag.String("web.telemetry-path", "/metrics", "Path under which to expose metrics.")
)

func main() {
	flag.Parse()

	if *flagVersion {
		fmt.Println(version.Print(name))
		return
	}

	switch *flagLogFormat {
	case "json":
		logrus.SetFormatter(&logrus.JSONFormatter{})
	default:
		logrus.SetFormatter(&logrus.TextFormatter{})
	}

	if !*flagCheck {
		logrus.Infof("Starting %s v%s...", name, version.Version)
		if *flagDebug {
			logrus.SetLevel(logrus.DebugLevel)
			logrus.Debug("Enabling debug output")
		}
	}

	config := &Config{}
	err := config.LoadFromFile(*flagConfigFile)
	if err != nil {
		logrus.Fatal(err)
	}

	if *flagCheck {
		fmt.Println(config.String())
		return
	}

	exporter := NewVaultExporter(config)

	prometheus.MustRegister(version.NewCollector(config.PGW.Namespace))

	if *config.Telemetry.HTTPEnabled {
		prometheus.MustRegister(
			exporter.scrapeTime,
			exporter.totalScrapes,
			exporter.errors,
			exporter.duration,
			exporter.execHistogram,
		)
	}

	go func() {
		http.Handle(*flagMetricPath, promhttp.Handler())
		logrus.Printf("Providing metrics at %s%s", *flagListenAddress, *flagMetricPath)
		logrus.Fatal(http.ListenAndServe(*flagListenAddress, nil))
	}()

	exporter.Collect()
}
