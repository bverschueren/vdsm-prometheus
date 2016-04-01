FROM sdurrheimer/alpine-glibc

EXPOSE 8181

COPY vdsm-prometheus /

CMD /vdsm-prometheus
