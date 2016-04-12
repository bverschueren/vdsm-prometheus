# vdsm-prometheus
Go client for VDSM which exposes host and VM stats as prometheus metrics.

The main purpose of this client is to run as a sidecar on every host where VDSM
is running. It will use the VDSM TLS certificate found on the host to connect
to VDSM and reuses this certificate to allow secured access for Prometheus to
the VDSM metrics.

[![Build Status](https://travis-ci.org/rmohr/vdsm-prometheus.svg?branch=master)](https://travis-ci.org/rmohr/vdsm-prometheus)


## Use it

The easisest way is to install the RPMs from
[copr](https://copr.fedorainfracloud.org/coprs/rfenkhuber/vdsm-prometheus/).
After activating the `vdsm-prometheus` service the metrics are exposed at
`:8181/metrics`.

### CentOS
```bash
curl https://copr.fedorainfracloud.org/coprs/rfenkhuber/vdsm-prometheus/repo/epel-7/rfenkhuber-vdsm-prometheus-epel-7.repo > /etc/yum.repos.d/vdsm-prometheus.repo
yum install vdsm-prometheus
systemctl start vdsm-prometheus
```
### Fedora
```bash
dnf copr enable rfenkhuber/vdsm-prometheus
dnf install vdsm-prometheus
systemctl start vdsm-prometheus
```

### Firewall configuration

For `iptables`:
```bash
iptables -I INPUT -p tcp --dport 8181 -j ACCEPT
```

For `firewalld`:

```bash
firewall-cmd --zone=public --add-port=8181/tcp
```

### Disable TLS for Prometheus

When trying out Prometheus it can be handy to allow Prometheus to access the
metrics without configuring a valid TLS certificate.  To do this, either run
`vdsm-prometheus -no-verify -no-prom-auth` directly or let systemd do the work
for you:

Add the following to
`/etc/systemd/system/vdsm-prometheus.service.d/10-vdsm-prometheus.conf`
(create the directory or file if it does not exist):
```ini
[Service]
Environment="VDSM_PROM_OPTS=-no-verify -no-prom-auth"
```
Then restart the service:
```bash
systemctl daemon-reload
systemctl restart vdsm-prometheus
```

`vdsm-prometheus` now still uses TLS when communicating with VDSM but
Prometheus can access the metrics without authentication.

Verify that TLS is disabled by running `curl localhost:8181/metrics` on the
VDSM host.

## Ansible

There exists an 
[Ansible role](https://galaxy.ansible.com/rmohr/vdsm-prometheus/) to ease the
installation of `vdsm-prometheus` on every oVirt managed host. Run

```bash
ansible-galaxy install rmohr.vdsm-prometheus
```

Now you can use the role in your playbooks. For instance to allow browsing the
metrics without TLS run a playbook like this:

```yaml
---
- hosts: vdsm
  roles:
    - { role: vdsm-prometheus, opts: "-no-verify -no-prom-auth" }
```

The playbook will install the the latest version of `vdsm-prometheus` from
[copr](https://copr.fedorainfracloud.org/coprs/rfenkhuber/vdsm-prometheus/),
configure systemd, the firewall and finally starts vdsm-prometheus.

## Hacking
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
