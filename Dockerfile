FROM resin/armv7hf-buildpack-deps:wheezy-scm

ENV DEBIAN_FRONTEND noninteractive

RUN apt-get update && apt-get install -qqy \
    avahi-daemon \
    dropbear \
    && apt-get clean \
    && rm -rf /var/lib/apt/lists

RUN echo "America/New_York" > /etc/timezone && dpkg-reconfigure -f noninteractive tzdata

ADD start.sh /start.sh

RUN chmod a+x /start.sh

CMD /start.sh
