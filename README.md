# vdsm-prometheus
Go client for VDSM which exposes host and VM stats as prometheus metrics.

## Run it
```bash
$ go build .
$ ./vdsm-prometheus -host {vdsm-ip}
```
Visit <http://localhost:8181/metrics> to find all the metrics exposed for
prometheus.

## Docker
Run
```bash
bash run_prometheus.sh {vdsm-ip}
```

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
