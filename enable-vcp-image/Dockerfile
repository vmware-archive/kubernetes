FROM ubuntu:16.10
MAINTAINER cna-storage@vmware.com


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
