FROM golang:1.14 as builder
WORKDIR /go/src/github.com/leominov/vault-reliability-exporter
COPY . .
RUN make build

FROM scratch
COPY --from=builder /go/src/github.com/leominov/vault-reliability-exporter/vault-reliability-exporter /go/bin/vault-reliability-exporter
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
ENTRYPOINT ["/go/bin/vault-reliability-exporter"]
