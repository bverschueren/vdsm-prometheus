FROM centos:7.2.1511

EXPOSE 8181

COPY vdsm-prometheus /

CMD /vdsm-prometheus
