go build .
docker build -t vdsm-prometheus .
docker stop vdsm-prometheus
docker stop prometheus
docker stop grafana
docker rm -v vdsm-prometheus
docker rm -v prometheus
docker rm -v grafana
docker run --name vdsm-prometheus -p 127.0.0.1:8181:8181 -d vdsm-prometheus /vdsm-prometheus -host=$1
docker run --name prometheus -p 9090:9090 --link vdsm-prometheus:vdsm-prometheus -v $PWD/prometheus.yml:/etc/prometheus/prometheus.yml:z -d prom/prometheus
docker run --name grafana -p 3000:3000 --link prometheus:prometheus -d grafana/grafana
