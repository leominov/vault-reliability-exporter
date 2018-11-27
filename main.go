package main

import (
	"flag"
	"fmt"
	"strings"
	"time"

	"github.com/prometheus/common/version"

	"github.com/sirupsen/logrus"
)

const (
	name = "Vault Reliability Exporter"
)

type Config struct {
	Namespace      string
	JobName        string
	PushgatewayURL string
	ClientTimeout  time.Duration
	Interval       time.Duration
	SecretPath     string
	Addr           string
	AuthMethod     string
	AuthLogin      string
	AuthPassword   string
	Labels         map[string]string
}

var (
	flagVersion = flag.Bool("version", false, "Prints version and exit.")

	flagLogFormat = flag.String("log-format", "txt", "Log format, valid options are txt and json.")
	flagDebug     = flag.Bool("debug", false, "Output verbose debug information.")

	flagPushgatewayURL      = flag.String("pushgateway.addr", "127.0.0.1:9091", "Pushgateway address.")
	flagExporterNamespace   = flag.String("namespace", "vault_reliability_exporter", "Namespace for metrics.")
	flagExporterJobName     = flag.String("job", "vault_reliability_job", "Job's name.")
	flagVaultAddr           = flag.String("vault.addr", "https://127.0.0.1:8200", "Vault address.")
	flagVaultClientTimeout  = flag.Duration("vault.timeout", 30*time.Second, "Vault client's timeout.")
	flagVaultAuthMetod      = flag.String("vault.auth-method", "userpass", "Vault user's auth method.")
	flagVaultAuthLogin      = flag.String("vault.auth-login", "", "Vault user's login.")
	flagVaultAuthPassw      = flag.String("vault.auth-password", "", "Vault user's password.")
	flagVaultRepeatInterval = flag.Duration("vault.repeat-interval", time.Second, "Checks repeat interval.")
	flagVaultSecretPath     = flag.String("vault.secret-path", "probe-secrets/test", "Vault secret path.")
	flagLabels              = flag.String("labels", "", "Comma-separated list of additional labels in format KEY=VALUE.")
)

func LabelStringToMap(inputLabels string) map[string]string {
	labels := make(map[string]string)
	if len(inputLabels) == 0 {
		return labels
	}
	labelsKV := strings.Split(inputLabels, ",")
	if len(labelsKV) == 0 {
		return labels
	}
	for _, labelKV := range labelsKV {
		label := strings.Split(labelKV, "=")
		if len(label) != 2 {
			continue
		}
		labels[label[0]] = label[1]
	}
	return labels
}

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

	logrus.Infof("Starting %s v%s...", name, version.Version)

	if *flagDebug {
		logrus.SetLevel(logrus.DebugLevel)
		logrus.Debug("Enabling debug output")
	}

	labels := LabelStringToMap(*flagLabels)
	logrus.Debugf("Additional labels: %v", labels)

	config := &Config{
		Namespace:      *flagExporterNamespace,
		JobName:        *flagExporterJobName,
		PushgatewayURL: *flagPushgatewayURL,
		ClientTimeout:  *flagVaultClientTimeout,
		Interval:       *flagVaultRepeatInterval,
		SecretPath:     *flagVaultSecretPath,
		Addr:           *flagVaultAddr,
		AuthMethod:     *flagVaultAuthMetod,
		AuthLogin:      *flagVaultAuthLogin,
		AuthPassword:   *flagVaultAuthPassw,
		Labels:         labels,
	}

	collector := NewVaultExporter(config)
	collector.Collect()
}
