FROM gliderlabs/alpine
MAINTAINER containers@vmware.com

RUN apk add --no-cache bash jq py-pip openssl go git libc-dev
ENV GOPATH /go
ENV PATH $GOPATH/bin:/usr/local/go/bin:$PATH
RUN mkdir -p "$GOPATH/src" "$GOPATH/bin" && chmod -R 777 "$GOPATH"

#### ---- Installer Files ---- ####
ARG INSTALLER=install.sh
ADD $INSTALLER /tmp/
RUN chmod +x /tmp/$INSTALLER && \ 
    /tmp/$INSTALLER

ADD enable-vcp-scripts /opt/enable-vcp-scripts
RUN chmod +x /opt/enable-vcp-scripts/run.sh && \
    chmod +x /opt/enable-vcp-scripts/daemonset_pod.sh && \
    chmod +x /opt/enable-vcp-scripts/manager_pod.sh

# execute run.sh when the image starts
CMD [ "/opt/enable-vcp-scripts/run.sh" ]
