package main

import (
	"bufio"
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
	useLibCurl             bool
	useShortOpt            bool
	verbose                bool
}

type request struct {
	method    string
	path      string
	http      string
	header    map[string]string
	exactCase map[string]string
	body      string
}

var opt option
var requestLineRe = regexp.MustCompile(`([^ ]*) +(.*) +(HTTP/.*)`)
var headerRe = regexp.MustCompile(`([^:]*): *(.*)`)

func init() {
	flag.BoolVar(&opt.help, "h", false, "Show help")
	flag.BoolVar(&opt.allowCurlDefaultHeader, "a", true, "Allow curl's default headers")
	flag.BoolVar(&opt.document, "d", false, "Output man page HTML links after command line")
	flag.BoolVar(&opt.useHTTP, "H", false, "Output HTTP generated URLs instead")
	flag.BoolVar(&opt.ignoreHTTPVersion, "i", true, "Ignore HTTP version")
	flag.BoolVar(&opt.useLibCurl, "libcurl", false, "Output libcurl code instead")
	flag.BoolVar(&opt.useShortOpt, "s", false, "Use short command line options")
	flag.BoolVar(&opt.verbose, "v", false, "Add a verbose option to the command line")
}

func main() {
	os.Exit(_main())
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
	var bodyBuf strings.Builder
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

		bodyBuf.WriteString(line)
		bodyBuf.WriteRune('\n')
	}

	if _, ok := req.header["host"]; !ok {
		return nil, fmt.Errorf("no host: header makes it impossible to tell URL")
	}

	if bodyBuf.Len() > 0 {
		req.body = bodyBuf.String()
		req.body = strings.TrimRight(req.body, "\n")
	}

	return &req, nil
}

func _main() int {
	_, err := readInput(os.Stdin)
	if err != nil {
		fmt.Printf("failed to parse input: %v", err)
		return 1
	}

	return 0
}
