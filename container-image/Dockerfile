FROM ubuntu:16.04
ENV TZ 'America/Los_Angeles'
    RUN echo $TZ > /etc/timezone && \
    apt-get update && apt-get install -y tzdata && \
    rm /etc/localtime && \
    ln -snf /usr/share/zoneinfo/$TZ /etc/localtime && \
    dpkg-reconfigure -f noninteractive tzdata && \
    apt-get clean

ENV GOVERSION 1.8.3
ENV GOROOT /opt/go
ENV GOPATH /root/.go

RUN apt-get update
RUN apt-get install -y openssh-server git make curl uuid-runtime genisoimage net-tools binfmt-support vim expect ssh
RUN apt-get clean

RUN cd /opt && wget https://get.docker.com/builds/Linux/x86_64/docker-1.12.6.tgz && \
    tar zxf docker-1.12.6.tgz && rm docker-1.12.6.tgz && \
    chmod +x /opt/docker/docker && \
    ln -s /opt/docker/docker /usr/bin/

RUN cd /opt && wget https://storage.googleapis.com/golang/go${GOVERSION}.linux-amd64.tar.gz && \
    tar zxf go${GOVERSION}.linux-amd64.tar.gz && rm go${GOVERSION}.linux-amd64.tar.gz && \
    ln -s /opt/go/bin/go /usr/bin/ && \
    mkdir $GOPATH

RUN go get github.com/vmware/govmomi/govc && \
    ln -s /root/.go/bin/govc /usr/bin

RUN mkdir -p /root/scripts
ADD configre_passwordless_login.sh /root/scripts/configure_passwordless_login.sh
ADD copy_pub_key.exp /root/scripts/copy_pub_key.exp

