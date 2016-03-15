# headscan

Scan a list of domains to see if they return a specific HTTP header

# Usage

headscan is used to test one of more origin web servers to see if they
return a specific HTTP header.

It expects to receive one or more lines on stdin that consist of comma
separated entries representing an HTTP Host header value and the name
of an origin web server to which to send an HTTP request. For example,

     echo "www.cloudflare.com,cloudflare.com" | ./headscan -header=Cookie

would connect to cloudflare.com and do a GET for / with the Host
header set to www.cloudflare.com and check to see if the server
returned a Cookie header.

headscan outputs one comma-separated line per input line.

For example, the above might output:

     cloudflare.com,www.cloudflare.com,t,f

Breaking that down:

`cloudflare.com,` Origin server contacted

`www.cloudflare.com,` Host header sent

`t,` t if the origin server name resolved

`f,` t if a Cookie header was present, f if not

# Options

`-header` Sets the HTTP header to look for; must be present

`-fields` If set outputs a header line containing field names
		
`-log` File to write log information to
		
`-resolver` DNS resolver address (default 127.0.0.1)

`-workers` Number of concurrent workers (default 10)

