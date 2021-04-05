FROM alpine
EXPOSE 9876
WORKDIR /
COPY speedtest-exporter*.apk /tmp
RUN apk add --allow-untrusted /tmp/speedtest-exporter*.apk
ENTRYPOINT ["/usr/local/bin/speedtest-exporter"]
