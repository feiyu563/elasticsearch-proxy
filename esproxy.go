package main

import (
	"bytes"
	"flag"
	"fmt"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)
var debugLog,errorLog,infoLog  *log.Logger
var ProxyUrl1,ProxyUrl2,Port  string
var RetryTime  int
var TimeOut int64
var help bool

var (
	Proxy_receiver_total = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "proxy_receiver_total",
		Help: "count proxy receiver request",
	},
		[]string {"Server","Method","Uri"},
	)
	//model.AlertsFromCounter.WithLabelValues("from","to","message","level","host","index").Add(1)
	Proxy_send_success_total = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "proxy_send_success_total",
		Help: "count proxy send success request",
	},
		[]string {"Server","Method","Uri"},
	)
	//model.AlertToCounter.WithLabelValues("to","message").Add(1)
	Proxy_send_fail_total = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "proxy_send_fail_total",
		Help: "count proxy fail request",
	},
		[]string {"Server","Method","Uri"},
	)
)

func init()  {
	flag.StringVar(&ProxyUrl1, "hosta", "http://10.25.60.212:9200", "指定默认代理的目标`http://10.25.60.212:9200`")
	flag.StringVar(&ProxyUrl2, "hostb", "http://10.25.60.213:9200", "指定流量镜像的目标`http://10.25.60.213:9200`")
	flag.StringVar(&Port, "p", "9200", "代理服务监听端口`9200`")
	flag.IntVar(&RetryTime, "r", 3, "`后端代理失败重试次数`")
	flag.Int64Var(&TimeOut, "t", 30, "`后端代理超时时间(秒)`")
	flag.BoolVar(&help,"h", false, "显示帮助")
	flag.Usage = usage
}
func usage() {
	fmt.Fprintf(os.Stderr, `Version 0.1 If you need help contact jikun.zhang@megatronix.co
Usage: esproxy [-h] [-hosta ProxyTarget] [-hostb CopyTarget] [-p Port] [-r RetryTime] [-t TimeOut]
Example：esproxy -hosta http://10.25.60.212:9200 -hostb http://10.25.60.213:9200 -p 9200 -r 3 -t 30

Options:
`)
	flag.PrintDefaults()
}

func main() {
	flag.Parse()
	if help {
		flag.Usage()
		return
	}
	infoFile, err := os.OpenFile("log/proxy.log", os.O_WRONLY | os.O_CREATE | os.O_APPEND, 0644)
	if err != nil {
		fmt.Println("[ "+time.Now().Format("2006/01/02 15:04:05")+" ]",err.Error())
	}
	errorFile, err := os.OpenFile("log/proxy-error.log", os.O_WRONLY | os.O_CREATE | os.O_APPEND, 0644)
	if err != nil {
		fmt.Println("[ "+time.Now().Format("2006/01/02 15:04:05")+" ]",err.Error())
	}
	debugLog = log.New(infoFile, "[DEBUG] [ ", log.Ldate|log.Ltime )
	errorLog = log.New(errorFile, "[ERROR] [ ", log.Ldate|log.Ltime)
	infoLog = log.New(infoFile, "[INFO] [ ", log.Ldate|log.Ltime)
	prometheus.MustRegister(Proxy_receiver_total,Proxy_send_success_total,Proxy_send_fail_total)
	http.HandleFunc("/", handler)
	http.Handle("/app/metrics", promhttp.Handler())
	fmt.Println("[ "+time.Now().Format("2006/01/02 15:04:05")+" ] Start Elasticsearch Proxy On 0.0.0.0:"+Port)
	http.ListenAndServe(":"+Port, nil)
	os.Exit(0)
}

func handler(w http.ResponseWriter, r *http.Request) {
	//var tr *http.Transport
	//tr = &http.Transport{
	//	TLSClientConfig:    &tls.Config{InsecureSkipVerify: true},
	//	Proxy: proxy,
	//}

	//超时时间设置
	HttpTimeOut:=time.Duration(TimeOut)*time.Second
	client := &http.Client{Timeout:HttpTimeOut}
	HttpBody,_:=ioutil.ReadAll(r.Body)
	//缓存request body
	ReqBody1:=ioutil.NopCloser(bytes.NewBuffer(HttpBody))
	ReqBody2:=ioutil.NopCloser(bytes.NewBuffer(HttpBody))
	//记录请求内容
	Proxy_receiver_total.WithLabelValues("127.0.0.1:9200",r.Method,r.RequestURI).Add(1)
	infoLog.Println("] [Server] 127.0.0.1:9200 [Method] "+r.Method," [Uri] "+r.RequestURI,"[HttpBody] "+strings.Replace(string(HttpBody),"\n","",-1))

	//异步转发给第二个ES

	infoLog.Println("] [Server] "+ProxyUrl2, "[Method] "+r.Method, " [Uri] "+r.RequestURI, "[HttpBody] "+strings.Replace(string(HttpBody), "\n", "", -1))
	channel := make(chan int)
	go func(ch chan int) {
		for Retry:=0; Retry<RetryTime+1;Retry++  {
			req2, err := http.NewRequest(r.Method,ProxyUrl2+r.RequestURI,ReqBody2)
			req2.Header=r.Header
			_, err = client.Do(req2)
			if err != nil {
				Proxy_send_fail_total.WithLabelValues(ProxyUrl2,r.Method,r.RequestURI).Add(1)
				infoLog.Println("] [Server] "+ProxyUrl2,"[Method] "+r.Method," [Uri] "+r.RequestURI,err.Error())
				if Retry == RetryTime {
					errorLog.Println("] [Server] "+ProxyUrl2, "[Method] "+r.Method, " [Uri] "+r.RequestURI, "[HttpBody] "+strings.Replace(string(HttpBody), "\n", "", -1))
					break
				}
				debugLog.Println("] [Server] "+ProxyUrl2,"[Method] "+r.Method," [Uri] "+r.RequestURI," retry time "+strconv.Itoa(Retry+1))
			}else {
				Proxy_send_success_total.WithLabelValues(ProxyUrl2,r.Method,r.RequestURI).Add(1)
				break
			}
			defer req2.Body.Close()
		}
		ch <- 1
		<- channel
	}(channel)


	//请求第一个ES
	var resp  = &http.Response{}
	infoLog.Println("] [Server] "+ProxyUrl1, "[Method] "+r.Method, " [Uri] "+r.RequestURI, "[HttpBody] "+strings.Replace(string(HttpBody), "\n", "", -1))
	for Retry:=0; Retry<RetryTime+1;Retry++  {
		req, err := http.NewRequest(r.Method,ProxyUrl1+r.RequestURI,ReqBody1)
		req.Header=r.Header
		resp, err = client.Do(req)
		if err != nil {
			Proxy_send_fail_total.WithLabelValues(ProxyUrl1,r.Method,r.RequestURI).Add(1)
			infoLog.Println("] [Server] "+ProxyUrl1,"[Method] "+r.Method," [Uri] "+r.RequestURI,err.Error())
			if Retry == RetryTime {
				http.NotFound(w, r)
				errorLog.Println("] [Server] "+ProxyUrl1, "[Method] "+r.Method, " [Uri] "+r.RequestURI, "[HttpBody] "+strings.Replace(string(HttpBody), "\n", "", -1))
				return
			}
			debugLog.Println("] [Server] "+ProxyUrl1,"[Method] "+r.Method," [Uri] "+r.RequestURI," retry time "+strconv.Itoa(Retry+1))
		}else {
			Proxy_send_success_total.WithLabelValues(ProxyUrl1,r.Method,r.RequestURI).Add(1)
			break
		}
		defer req.Body.Close()
	}

	//处理返回
	data, err := ioutil.ReadAll(resp.Body)
	if err != nil && err != io.EOF {
		http.NotFound(w, r)
		return
	}
	for i, j := range resp.Header {
		for _, k := range j {
			w.Header().Add(i, k)
		}
	}
	for _, c := range resp.Cookies() {
		w.Header().Add("Set-Cookie", c.Raw)
	}
	_, ok := resp.Header["Content-Length"]
	if !ok && resp.ContentLength > 0 {
		w.Header().Add("Content-Length", fmt.Sprint(resp.ContentLength))
	}
	w.WriteHeader(resp.StatusCode)
	w.Write(data)
}