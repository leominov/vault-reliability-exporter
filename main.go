package main

import (
	"flag"
	"time"

	"github.com/sirupsen/logrus"
)

type Config struct {
	Namespace      string
	JobName        string
	PushgatewayURL string
	Interval       time.Duration
	SecretPath     string
	Addr           string
	AuthMethod     string
	AuthLogin      string
	AuthPassword   string
}

var (
	flagLogFormat = flag.String("log-format", "txt", "Log format, valid options are txt and json.")
	flagDebug     = flag.Bool("debug", false, "Output verbose debug information.")

	flagPushgatewayURL      = flag.String("pushgateway.addr", "", "Pushgateway address.")
	flagExporterNamespace   = flag.String("namespace", "vault_reliability_exporter", "Namespace for metrics.")
	flagExporterJobName     = flag.String("job", "vault_reliability_job", "Job's name.")
	flagVaultAddr           = flag.String("vault.addr", "", "Vault address.")
	flagVaultAuthMetod      = flag.String("vault.auth-method", "userpass", "Vault user's auth method.")
	flagVaultAuthLogin      = flag.String("vault.auth-login", "", "Vault user's login.")
	flagVaultAuthPassw      = flag.String("vault.auth-password", "", "Vault user's password.")
	flagVaultRepeatInterval = flag.Duration("vault.repeat-interval", time.Second, "Checks repeat interval.")
	flagVaultSecretPath     = flag.String("vault.secret-path", "probe-secrets/test", "Vault secret path")
)

func main() {
	flag.Parse()

	switch *flagLogFormat {
	case "json":
		logrus.SetFormatter(&logrus.JSONFormatter{})
	default:
		logrus.SetFormatter(&logrus.TextFormatter{})
	}

	logrus.Info("Starting Vault Reliability Exporter...")

	if *flagDebug {
		logrus.SetLevel(logrus.DebugLevel)
		logrus.Debug("Enabling debug output")
	}

	config := &Config{
		Namespace:      *flagExporterNamespace,
		JobName:        *flagExporterJobName,
		PushgatewayURL: *flagPushgatewayURL,
		Interval:       *flagVaultRepeatInterval,
		SecretPath:     *flagVaultSecretPath,
		Addr:           *flagVaultAddr,
		AuthMethod:     *flagVaultAuthMetod,
		AuthLogin:      *flagVaultAuthLogin,
		AuthPassword:   *flagVaultAuthPassw,
	}

	collector := NewVaultExporter(config)
	collector.Collect()
}
