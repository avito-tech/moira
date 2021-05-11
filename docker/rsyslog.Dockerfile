FROM registry.k.avito.ru/avito/debian-minbase:stretch

ENV PACKAGE_VERSION=8.24.0-1

RUN apt-get update \
  && apt-get install -y \
    rsyslog=$PACKAGE_VERSION \
    rsyslog-czmq=$PACKAGE_VERSION \
  && rm -rf /var/lib/apt/lists/* /tmp/* \
  && apt-get clean

VOLUME /var/run/rsyslog/dev
EXPOSE 514/tcp 514/udp

CMD [ "rsyslogd", "-f", "/etc/rsyslog.conf", "-n" ]
