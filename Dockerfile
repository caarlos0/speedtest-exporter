FROM ubuntu
EXPOSE 9876
WORKDIR /
RUN apt update && \
	apt install -y gnupg1 apt-transport-https dirmngr && \
	apt-key adv --keyserver keyserver.ubuntu.com --recv-keys 379CE192D401AB61 && \
	echo "deb https://ookla.bintray.com/debian generic main" | tee  /etc/apt/sources.list.d/speedtest.list && \
	apt update && \
	apt install -y speedtest
COPY speedtest-exporter*.deb /tmp
RUN dpkg -i /tmp/speedtest-exporter*.deb
ENTRYPOINT ["/usr/local/bin/speedtest-exporter"]
