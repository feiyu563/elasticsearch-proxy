FROM centos:7
WORKDIR /opt
RUN mkdir -p /opt/log
ADD esproxy /opt/
CMD ["/opt/esproxy", "-hosta", "http://10.25.60.200:9200", "-hostb", "http://172.16.3.144:9200", "-p", "9222", "-r", "3", "-t", "30"]