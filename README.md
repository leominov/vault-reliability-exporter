# HashiCorp Vault Reliability Exporter

## Usage

```
Usage of exporter:
  -debug
    	Output verbose debug information.
  -job string
    	Job's name. (default "vault_reliability_job")
  -log-format string
    	Log format, valid options are txt and json. (default "txt")
  -namespace string
    	Namespace for metrics. (default "vault_reliability")
  -pushgateway.addr string
    	Pushgateway address.
  -vault.addr string
    	Vault address.
  -vault.auth-login string
    	Vault user's login.
  -vault.auth-method string
    	Vault user's auth method. (default "userpass")
  -vault.auth-password string
    	Vault user's password.
  -vault.repeat-interval duration
    	Checks repeat interval. (default 1s)
```

## Metrics

* `vault_reliability_execution_time_bucket` by le, type
* `vault_reliability_exporter_scrape_time`
* `vault_reliability_exporter_scrapes_total`
* `vault_reliability_exporter_auth_error_total`
* `vault_reliability_exporter_read_error_total`
* `vault_reliability_exporter_write_error_total`
* `vault_reliability_exporter_last_scrape_duration_seconds`
