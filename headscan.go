// headscan is used to test one of more origin web servers to see if
// they return a specific HTTP header.
//
// It expects to receive one or more lines on stdin that consist of
// comma separated entries representing an HTTP Host header value and
// the name of an origin web server to which to send an HTTP
// request. For example,
//
//  echo "www.cloudflare.com,cloudflare.com" | ./headscan -header=Cookie
//
// would connect to cloudflare.com and do a GET for / with the Host
// header set to www.cloudflare.com. The origin can be an IP address.
//
// headscan outputs one comma-separated line per input line.
//
// For example, the above might output:
//
//     cloudflare.com,www.cloudflare.com,t,t
//
// Breaking that down:
//
// cloudflare.com,           Origin server contacted
// www.cloudflare.com,       Host header sent
// t,                        t if the origin server name resolved
// t                         t indicates that the Cookie header was present

package main

import (
	"bufio"
	"flag"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/bogdanovich/dns_resolver"
)

// The HTTP header to look for
var header *string

var resolverName string

// tri captures a tri-state. The value of yesno is true only is ran is
// true
type tri struct {
	ran   bool
	yesno bool
}

func (t tri) String() string {
	switch {
	case !t.ran:
		return "-"
	case t.yesno:
		return "t"
	case !t.yesno:
		return "f"
	}

	// Should not be reached ever

	return "!"
}

// site is a web site identified by its DNS name along with the state
// of various tests performed on the site.
type site struct {
	host   string // Host header that needs to be set
	origin string // DNS name of the web site

	resolves tri // Whether the name resolves
	present  tri // Whether the header was present
}

// test tests a site and looks for the header
func (s *site) test(l *os.File) {
	resolver := dns_resolver.New([]string{resolverName})

	// Check that the origin server resolves

	s.resolves.ran = true
	name := s.origin
	if net.ParseIP(name) == nil {
		_, err := resolver.LookupHost(name)
		if err != nil {
			s.logf(l, "Error resolving name: %s", err)
			s.resolves.yesno = false
			return
		}
	}
	s.resolves.yesno = true

	// Custom dialer is needed to use special DNS resolver so that the
	// default resolver can be overriden

	transport := &http.Transport{}
	transport.Dial = func(network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, err
		}

		if net.ParseIP(host) != nil {
			return net.Dial(network, address)
		}

		ips, err := resolver.LookupHost(host)
		if err != nil {
			return nil, err
		}

		if len(ips) == 0 {
			return nil, fmt.Errorf("Failed to get any IPs for %s", address)
		}

		return net.Dial(network, net.JoinHostPort(ips[0].String(), port))
	}

	client := &http.Client{Transport: transport}
	req, err := http.NewRequest("GET", "http://"+name, nil)

	req.Header.Set("Accept-Encoding", "gzip,deflate")
	req.Header.Set("Host", s.host)

	s.present.ran = true
	resp, err := client.Do(req)
	if err != nil {
		s.logf(l, "HTTP request %#v failed: %s", req, err)
		return
	}
	s.present.yesno = resp.Header.Get(*header) != ""
	if resp != nil && resp.Body != nil {
		ioutil.ReadAll(resp.Body)
		resp.Body.Close()
	}
}

// logf writes to the log file prefixing with the origin being logged
func (s *site) logf(f *os.File, format string, a ...interface{}) {
	if f != nil {
		fmt.Fprintf(f, fmt.Sprintf(s.origin+": "+format+"\n", a...))
	}
}

// fields returns the list of fields that String() will return for a
// site
func (s *site) fields() string {
	return "origin,host,resolves,present"
}

func (s *site) String() string {
	return fmt.Sprintf("%s,%s,%s,%s", s.origin, s.host, s.resolves, s.present)
}

var wg sync.WaitGroup

func worker(work, result chan *site, l *os.File) {
	for s := range work {
		s.test(l)
		result <- s
	}
	wg.Done()
}

func writer(result chan *site, stop chan struct{}, fields bool) {
	first := true
	for s := range result {
		if fields && first {
			fmt.Printf("%s\n", s.fields())
			first = false
		}

		fmt.Printf("%s\n", s)
	}
	close(stop)
}

func main() {
	resolver := flag.String("resolver", "127.0.0.1", "DNS resolver address")
	header = flag.String("header", "", "HTTP header to look for")
	workers := flag.Int("workers", 10, "Number of concurrent workers")
	log := flag.String("log", "", "File to write log information to")
	fields := flag.Bool("fields", false,
		"If set outputs a header line containing field names")
	flag.Parse()

	if *header == "" {
		fmt.Println("-header must be present")
		return
	}

	*header = http.CanonicalHeaderKey(*header)

	if *workers < 1 {
		fmt.Println("-workers must be a positive number")
		return
	}

	resolverName = *resolver

	var l *os.File
	var err error
	if *log != "" {
		if l, err = os.Create(*log); err != nil {
			fmt.Printf("Failed to create log file %s: %s\n", *log, err)
			return
		}
		defer l.Close()
	}

	work := make(chan *site)
	result := make(chan *site)
	stop := make(chan struct{})

	go writer(result, stop, *fields)

	for i := 0; i < *workers; i++ {
		wg.Add(1)
		go worker(work, result, l)
	}

	scan := bufio.NewScanner(os.Stdin)
	for scan.Scan() {
		parts := strings.Split(scan.Text(), ",")
		if len(parts) != 2 {
			fmt.Printf("Bad line: %s\n", scan.Text())
		} else {
			work <- &site{host: parts[0], origin: parts[1]}
		}
	}

	close(work)
	wg.Wait()
	close(result)
	<-stop

	if scan.Err() != nil {
		fmt.Printf("Error reading input: %s\n", scan.Err())
		return
	}
}
