package main

import (
	"bufio"
	"crypto/tls"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strconv"
	"strings"
)

func main() {
	// read VCAP_APP_HOST and PORT env variables set by CloudFoundry
	host := os.Getenv("VCAP_APP_HOST")
	if len(host) == 0 {
		host = "0.0.0.0"
	}
	port := os.Getenv("PORT")
	if len(port) == 0 {
		port = "8080"
	}
	address := host + ":" + port

	// register proxy
	http.Handle("/", NewProxy())

	log.Printf("Starting ip-whitelist demo app, listening on [%s] ...\n", address)
	if err := http.ListenAndServe(address, nil); err != nil {
		log.Fatal(err)
	}
}

type Proxy struct {
	SkipSSLValidation bool
	AllowedIPs        []string
}

func NewProxy() *Proxy {
	skipSSLEnvValue := os.Getenv("SKIP_SSL_VALIDATION")
	if len(skipSSLEnvValue) == 0 {
		skipSSLEnvValue = "false"
	}
	skipSSL, _ := strconv.ParseBool(skipSSLEnvValue)

	// get list of allowed IPs
	file, err := os.Open("ip-whitelist.conf")
	if err != nil {
		log.Fatalf("failed to open file: %s", err)
	}
	allowedIPs := make([]string, 0)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		allowedIPs = append(allowedIPs, strings.TrimSpace(scanner.Text()))
	}
	file.Close()

	return &Proxy{
		SkipSSLValidation: skipSSL,
		AllowedIPs:        allowedIPs,
	}
}

func (p *Proxy) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	p.ReverseProxy(rw, req)
}

func (p *Proxy) ReverseProxy(rw http.ResponseWriter, req *http.Request) {
	log.Printf("proxying request: [%s; %s; %s]\n", req.Method, req.RequestURI, req.UserAgent())
	req.Header.Set("X-IP-Whitelisting-Proxy", "X-IP-Whitelisting-Proxy")

	// X-CF-Forwarded-Url is required to determine the target of the request after it has been passed the route service
	// https://docs.cloudfoundry.org/services/route-services.html#headers
	targetURL := req.Header.Get("X-CF-Forwarded-Url")
	if len(targetURL) == 0 {
		rw.WriteHeader(http.StatusBadRequest)
		_, _ = rw.Write([]byte("Bad Request"))
		return
	}

	target, err := url.Parse(targetURL)
	if err != nil {
		log.Println(err.Error())
		rw.WriteHeader(http.StatusBadRequest)
		_, _ = rw.Write([]byte("Bad Request: " + err.Error()))
		return
	}

	// block/allow IPs
	var found bool
	for _, allowedIP := range p.AllowedIPs {
		sourceIP := req.Header.Get("X-Forwarded-For")
		ips := strings.SplitN(sourceIP, ",", 2)
		if len(ips) > 1 && len(ips[0]) > 0 {
			sourceIP = strings.TrimSpace(ips[0])
		}

		if sourceIP == allowedIP {
			found = true
			break
		}
		if strings.Contains(allowedIP, "/") {
			_, subnet, _ := net.ParseCIDR(allowedIP)
			ip := net.ParseIP(sourceIP)
			if subnet.Contains(ip) {
				found = true
				break
			}
		}
	}
	if !found {
		log.Printf("blocking request from [%s]", req.Header.Get("X-Forwarded-For"))
		rw.WriteHeader(http.StatusForbidden)
		_, _ = rw.Write([]byte("Forbidden"))
		return
	}

	// setup a reverse proxy and forward the original request to the target
	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.Transport = &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: p.SkipSSLValidation},
	}

	req.URL.Scheme = target.Scheme
	req.URL.Host = target.Host
	req.Host = target.Host

	proxy.ServeHTTP(rw, req)
}
