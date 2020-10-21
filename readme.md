esproxy基本使用

- esproxy支持功能

1.代理http流量到指定的多个后端elasticsearch.默认仅返回主(hosta主机)的response消息给客户端.

2.记录所有HTTP协议消息，存于单独的日志文件中.

3.对于失败的请求（主要是网络问题导致的失败）进行重试，如重试依旧失败，将失败的请求记录到单独的日志中存档。

4.支持metrics,接口`/app/metrics`.导出指标：`proxy_receiver_total`、`proxy_send_success_total`、`proxy_send_fail_total`


```
[root@k8s-master01 linux]# esproxy -h
Usage: esproxy [-h] [-hosta ProxyTarget] [-hostb CopyTarget] [-p Port] [-r RetryTime] [-t TimeOut] 
Example：esproxy -hosta http://10.25.60.212:9200 -hostb http://10.25.60.213:9200 -p 9200 -r 3 -t 30
 
hosta— 指定默认代理的目标
hostb— 指定流量镜像的目标
p----  代理服务监听端口
r----  后端代理失败重试次数
t----  后端代理超时时间(秒)
```

