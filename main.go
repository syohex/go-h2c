package main

import (
	"bufio"
	"encoding/base64"
	"flag"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
)

type option struct {
	help                   bool
	allowCurlDefaultHeader bool
	document               bool
	useHTTP                bool
	ignoreHTTPVersion      bool
	useShortOpt            bool
	verbose                bool
}

type request struct {
	method    string
	path      string
	http      string
	header    map[string]string
	exactCase map[string]string
	body      []string
}

type curlFlags struct {
	data      string
	request   string
	head      string
	header    string
	userAgent string
	cookie    string
	verbose   string
	form      string
	user      string
}

var opt option
var curlOption curlFlags
var requestLineRe = regexp.MustCompile(`([^ ]*) +(.*) +(HTTP/.*)`)
var headerRe = regexp.MustCompile(`([^:]*): *(.*)`)
var basicHeaderRe = regexp.MustCompile(`^Basic (.*)`)

func init() {
	flag.BoolVar(&opt.help, "h", false, "Show help")
	flag.BoolVar(&opt.allowCurlDefaultHeader, "a", false, "not allow curl's default headers")
	flag.BoolVar(&opt.document, "d", false, "Output man page HTML links after command line")
	flag.BoolVar(&opt.useHTTP, "H", false, "Output HTTP generated URLs instead")
	flag.BoolVar(&opt.ignoreHTTPVersion, "i", false, "Ignore HTTP version")
	flag.BoolVar(&opt.useShortOpt, "s", false, "Use short command line options")
	flag.BoolVar(&opt.verbose, "v", false, "Add a verbose option to the command line")

	setCurlFlags()
}

func main() {
	os.Exit(_main())
}

func setCurlFlags() {
	if !opt.useShortOpt {
		curlOption.data = "--data"
		curlOption.request = "--request"
		curlOption.head = "--head"
		curlOption.header = "--header"
		curlOption.userAgent = "--user-agent"
		curlOption.cookie = "--cookie"
		curlOption.verbose = "--verbose"
		curlOption.form = "--form"
		curlOption.user = "--user"
	} else {
		curlOption.data = "-d"
		curlOption.request = "-X"
		curlOption.head = "-I"
		curlOption.header = "-H"
		curlOption.userAgent = "-A"
		curlOption.cookie = "-b"
		curlOption.verbose = "-v"
		curlOption.form = "-F"
		curlOption.user = "-u"
	}
}

func isSupportedMethod(method string) bool {
	m := strings.ToUpper(method)
	return m == "GET" || m == "POST" || m == "HEAD" || m == "PUT" || m == "OPTIONS"
}

func readInput(r io.Reader) (*request, error) {
	const (
		stateRequestLine = iota
		stateHeader
		stateBody
	)

	var req request
	req.header = make(map[string]string)
	req.exactCase = make(map[string]string)

	state := stateRequestLine
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()
		if state == stateRequestLine {
			match := requestLineRe.FindStringSubmatch(line)
			if match == nil {
				return nil, fmt.Errorf("bad request-line")
			}

			req.method = match[1]
			req.path = match[2]
			req.http = match[3]
			state = stateHeader
			continue
		}

		if state == stateHeader {
			match := headerRe.FindStringSubmatch(line)
			if len(match) > 0 {
				header := strings.ToLower(match[1])
				value := strings.ToLower(match[2])

				req.header[header] = value
				req.exactCase[header] = match[1]
			} else if len(line) < 2 {
				// read separator part
				state = stateBody
			} else {
				return nil, fmt.Errorf("illegal HTTP header on line: %s", line)
			}

			continue
		}

		req.body = append(req.body, line)
	}

	if _, ok := req.header["host"]; !ok {
		return nil, fmt.Errorf("no host: header makes it impossible to tell URL")
	}

	if !isSupportedMethod(req.method) {
		return nil, fmt.Errorf("unsupported HTTP method: '%s'", req.method)
	}

	hasContent := false
	for _, line := range req.body {
		if line != "" {
			hasContent = true
			break
		}
	}

	if !hasContent {
		req.body = nil
	}

	return &req, nil
}

func manpage(flag string, desc string) string {
	return fmt.Sprintf("%s;%s", flag, desc)
}

func needQuote(s string) bool {
	for _, r := range " &?" {
		if strings.ContainsRune(s, r) {
			return true
		}
	}

	return false
}

func httpProtocol() string {
	if opt.useHTTP {
		return "http"
	} else {
		return "https"
	}
}

func _main() int {
	req, err := readInput(os.Stdin)
	if err != nil {
		fmt.Printf("failed to parse input: %v", err)
		return 1
	}

	var bodyPart string
	var docs []string
	if strings.HasPrefix(req.header["content-type"], "multipart/form-data;") {
		// TODO
	} else if len(req.body) > 0 {
		body := strings.Join(req.body, "")
		body = strings.ReplaceAll(body, "\n", " ") // turn newlines into space
		body = strings.ReplaceAll(body, `"`, `\"`) // escape double quotes
		bodyPart = fmt.Sprintf(`--data-binary "%s" `, body)

		docs = append(docs, manpage("--data-binary", "send this string as a body with POST"))
	}

	var methodPart string
	var requestPart string
	m := strings.ToUpper(req.method)
	if m == "HEAD" {
		methodPart = curlOption.head + " "
		docs = append(docs, manpage(curlOption.head, "send a HEAD request"))
	} else if m == "POST" {
		if bodyPart == "" {
			bodyPart = fmt.Sprintf(`%s "" `, curlOption.data)
			docs = append(docs, manpage(curlOption.data, "send this string as a body with POST"))
		}
	} else if m == "PUT" {
		if bodyPart == "" {
			bodyPart = fmt.Sprintf(`%s "" `, curlOption.data)
			docs = append(docs, manpage(curlOption.data, "send this string as a body with POST"))
		}

		bodyPart += fmt.Sprintf("%s PUT ", curlOption.request)
		docs = append(docs, manpage(curlOption.request, "replace the request method with this string"))
	} else if m == "OPTIONS" {
		methodPart = fmt.Sprintf("%s OPTIONS ", curlOption.request)
		if !strings.HasPrefix(req.path, "/") {
			requestPart = fmt.Sprintf(`--request-target "%s" `, req.path)
			docs = append(docs, manpage("--request-target", "specify request target to use instead of using the URL's"))

			req.path = ""
		}
	}

	disabledHeaders := ""
	addedHeaders := ""
	if bodyPart != "" {
		if contentType, ok := req.header["content-type"]; ok {
			ct := strings.ToUpper(contentType)
			if ct == "application/x-www-form-urlencoded" {
				addedHeaders = fmt.Sprintf(`"Content-Type: %s"`, contentType)
			}
		} else {
			disabledHeaders = fmt.Sprintf("%s Content-Type: ", curlOption.header)
		}
	}

	httpVersion := ""
	if !opt.ignoreHTTPVersion {
		http := strings.ToUpper(req.http)
		if http == "HTTP/1.1" {
			httpVersion = "--http1.1 "
			docs = append(docs, manpage("--http1.1", "use HTTP protocol version 1.1"))
		} else if http == "HTTP/2" {
			httpVersion = "--http2 "
			docs = append(docs, manpage("--http2", "use HTTP protocol version 2"))
		} else {
			fmt.Printf("unsupported HTTP version: %s\n", req.http)
			return 1
		}
	}

	if !opt.allowCurlDefaultHeader {
		if _, ok := req.header["accept"]; !ok {
			disabledHeaders += fmt.Sprintf("%s Accept: ", curlOption.header)
		}
		if _, ok := req.header["user-agent"]; !ok {
			disabledHeaders += fmt.Sprintf("%s User-Agent: ", curlOption.header)
		}
	}

	for k, v := range req.header {
		key := strings.ToLower(k)
		if key == "host" {
			// we use Host: for the URL creation
		} else if key == "authorization" {
			match := basicHeaderRe.FindStringSubmatch(v)
			if len(match) > 0 {
				decoded, err := base64.StdEncoding.DecodeString(match[1])
				if err != nil {
					fmt.Printf("failed to decode authorization info: %v", err)
					return 1
				}

				addedHeaders += fmt.Sprintf(`%s "%s" `, curlOption.user, decoded)
				docs = append(docs, manpage(curlOption.user, "use this user and password for basic auth"))
			}
		} else if key == "expect" {
			// let curl do expect on its own
		} else if key == "accept-encoding" && strings.Contains(v, "gzip") {
			addedHeaders += "--compressed "
			docs = append(docs, manpage("--compressed", "request a compressed response"))
		} else if key == "accept" && v == "*/*" {
			// ignore if set to */* as that's a curl default
		} else if key == "content-length" {
			// we don't set custom size
		} else {
			var option string
			if key == "user-agent" {
				option = fmt.Sprintf(`%s "`, curlOption.userAgent)
				docs = append(docs, manpage(curlOption.userAgent, "use this custom User-Agent request Header"))
			} else if key == "cookie" {
				option = fmt.Sprintf(`%s "`, curlOption.cookie)
				docs = append(docs, manpage(curlOption.cookie, "pass on this custom Cookie: request header"))
			} else {
				exactName := req.exactCase[k]
				option = fmt.Sprintf(`%s "%s: `, curlOption.header, exactName)
			}

			addedHeaders += fmt.Sprintf(`%s%s" `, option, v)
		}
	}

	var urlPart string
	host := req.header["host"]
	if needQuote(req.path) {
		urlPart = fmt.Sprintf(`"%s://%s%s"`, httpProtocol(), host, req.path)
	} else {
		urlPart = fmt.Sprintf(`%s://%s%s`, httpProtocol(), host, req.path)
	}

	if disabledHeaders != "" || addedHeaders != "" {
		docs = append(docs, manpage(curlOption.header, "add, replace or remove HTTP headers from the request"))
	}

	verbosePart := ""
	if opt.verbose {
		verbosePart = curlOption.verbose + " "
		docs = append(docs, manpage(curlOption.verbose, "show verbose output"))
	}

	curlCmd := fmt.Sprintf("curl %s%s%s%s%s%s%s%s",
		verbosePart, methodPart, httpVersion, disabledHeaders, addedHeaders, bodyPart, requestPart, urlPart)

	fmt.Println(curlCmd)
	return 0
}
