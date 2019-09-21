package main

import (
	"flag"
	"fmt"

	"github.com/prometheus/common/version"

	"github.com/sirupsen/logrus"
)

const (
	name = "Vault Reliability Exporter"
)

var (
	flagVersion = flag.Bool("version", false, "Prints version and exit.")

	flagLogFormat  = flag.String("log-format", "txt", "Log format, valid options are txt and json.")
	flagDebug      = flag.Bool("debug", false, "Output verbose debug information.")
	flagConfigFile = flag.String("config", "/etc/vault-reliability-exporter/config.yaml", "Path to configuration file.")
	flagCheck      = flag.Bool("check", false, "Check configuration and exit.")
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

	collector := NewVaultExporter(config)
	collector.Collect()
}
