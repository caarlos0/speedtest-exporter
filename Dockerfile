FROM alpine
EXPOSE 9876
WORKDIR /
RUN wget -O /tmp/speedtest.tgz https://install.speedtest.net/app/cli/ookla-speedtest-1.0.0-$(apk info --print-arch)-linux.tgz && \
	tar xvfz /tmp/speedtest.tgz -C /usr/local/bin speedtest && \
    rm -rf /tmp/speedtest.tgz
COPY speedtest-exporter*.apk /tmp
RUN apk add --allow-untrusted /tmp/speedtest-exporter*.apk
ENTRYPOINT ["/usr/local/bin/speedtest-exporter"]
