FROM centos:7

ENV SHELL=/bin/bash

RUN \
  rpm --rebuilddb && yum clean all && \
  yum install -y epel-release && \
  yum install -y pciutils && \
  yum install -y wget && \
  yum install -y python-pip

ADD bin/numatestdp /numa-test-device-plugin 
