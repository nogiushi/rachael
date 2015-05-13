FROM resin/i386-buildpack-deps:wheezy-scm

ENV DEBIAN_FRONTEND noninteractive

# gcc for cgo
RUN apt-get update && apt-get install -qqy \
    avahi-daemon \
    dropbear \
    gcc libc6-dev gcc-multilib make \
    --no-install-recommends \
    && apt-get clean \
    && rm -rf /var/lib/apt/lists/*

RUN echo "America/New_York" > /etc/timezone && dpkg-reconfigure -f noninteractive tzdata

ENV GOLANG_VERSION 1.4.2

RUN curl -sSL https://golang.org/dl/go$GOLANG_VERSION.src.tar.gz \
		| tar -v -C /usr/src -xz

RUN cd /usr/src/go/src && ./make.bash --no-clean 2>&1

ENV PATH /usr/src/go/bin:$PATH

RUN mkdir -p /go/src /go/bin && chmod -R 777 /go
ENV GOPATH /go
ENV PATH /go/bin:$PATH
WORKDIR /go

COPY . /go/src/github.com/nogiushi/rachael
RUN go get github.com/nogiushi/rachael
RUN go install github.com/nogiushi/rachael

ADD start.sh /start.sh
RUN chmod a+x /start.sh

CMD /start.sh
