# vdsm-prometheus
Go client for VDSM which exposes host and VM stats as prometheus metrics.

The main purpose of this client is to run as a sidecar on every host where VDSM
is running. It will use the VDSM TLS certificate found on the host to connect
to VDSM and reuses this certificate to allow secured access for Prometheus to
the VDSM metrics.

[![Build Status](https://travis-ci.org/rmohr/vdsm-prometheus.svg?branch=master)](https://travis-ci.org/rmohr/vdsm-prometheus)

## Run it
The easiest way for development is to disable VDSM TLS authentication on the
target host. Then build vdsm-prometheus, disable TLS authentication and point
it to the target VDSM server.
```bash
$ go build .
$ ./vdsm-prometheus -host {vdsm-ip} -no-prom-auth -no-vdsm-auth
```
Visit <http://localhost:8181/metrics> to find all the metrics exposed for
prometheus.

## Docker
During development to test vdsm-prometheus with Prometheus and Grafana you can
run
```bash
bash run_prometheus.sh {vdsm-ip}
```
Make sure to disable TLS on the target VDSM host or to provide valid
certificates to vdsm-prometheus.

This will start vdsm-promehteus, prometheus and grafana in docker containers:

```bash
$ docker ps
CONTAINER ID   IMAGE             COMMAND                  PORTS                         NAMES
042b9f154a77   grafana/grafana   "/run.sh"                0.0.0.0:3000->3000/tcp        grafana
96e7fddb1b03   prom/prometheus   "/bin/prometheus -con"   0.0.0.0:9090->9090/tcp        prometheus
cb6416dbf15e   vdsm-prometheus   "/vdsm-prometheus -ho"   127.0.0.1:8181->8181/tcp      vdsm-prometheus
```
Visit <http://localhost:9090> to create prometheus queries with prometheus
directly. Visit <http://localhost:3000> and log into grafana.  Then add a
prometheus data source: Select <http://prometheus:9090> as prometheus location
and select `Proxy` since grafana has to connect to prometheus through docker.
