// The code below is a goreplay middleware used for regression testing current
// vs next Horizon version. The middleware system of goreplay is rather simple:
// it streams one of 3 message types to stdin: request (HTTP headers),
// original response and replayed response. On request we can modify the request
// and send it to stdout but we don't use this feature here: we send request
// to mirroring target as is. Finally, everything printed to stderr is the
// middleware log, this is where we put the information about the request if the
// diff is found.
//
// More information and diagrams about the middlewares can be found here:
// https://github.com/buger/goreplay/wiki/Middleware
package main

import (
	"bufio"
	"bytes"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/buger/goreplay/proto"
	"github.com/stellar/go-stellar-sdk/support/log"
)

// maxPerSecond defines how many requests should be checked at max per second
const maxPerSecond = 100

const (
	requestType          byte = '1'
	originalResponseType byte = '2'
	replayedResponseType byte = '3'
)

var lastCheck = time.Now()
var reqsCheckedPerSeq = 0
var pendingRequestsAdded, ignoredCount, diffsCount, okCount int64
var pendingRequests = make(map[string]*Request)

func main() {
	processAll(os.Stdin, os.Stderr, os.Stdout)
}

func processAll(stdin io.Reader, stderr, stdout io.Writer) {
	log.SetOut(stderr)
	log.SetLevel(log.InfoLevel)

	bufSize := 20 * 1024 * 1024 // 20MB
	scanner := bufio.NewScanner(stdin)
	buf := make([]byte, bufSize)
	scanner.Buffer(buf, bufSize)
	var maxPendingRequests = 2000

	for scanner.Scan() {
		encoded := scanner.Bytes()
		buf := make([]byte, len(encoded)/2)
		_, err := hex.Decode(buf, encoded)
		if err != nil {
			os.Stderr.WriteString(fmt.Sprintf("hex.Decode error: %v", err))
			continue
		}

		if err := scanner.Err(); err != nil {
			os.Stderr.WriteString(fmt.Sprintf("scanner.Err(): %v\n", err))
		}

		process(stderr, stdout, buf)

		if len(pendingRequests) > maxPendingRequests {
			// Around 3-4% of responses is lost (not sure why) so pendingRequests can grow
			// indefinitely. Let's just truncate it when it becomes too big.
			// There is one gotcha here. Goreplay will still send requests
			// (`1` type payloads) even if traffic is rate limited. So if rate
			// limit is applied even more requests can be lost. So we should
			// use rate limiting implemented here when using middleware rather than
			// Goreplay's rate limit.
			pendingRequests = make(map[string]*Request)
		}
	}
}

func process(stderr, stdout io.Writer, buf []byte) {
	// First byte indicate payload type:
	payloadType := buf[0]
	headerSize := bytes.IndexByte(buf, '\n') + 1
	header := buf[:headerSize-1]

	// Header contains space separated values of: request type, request id, and request start time (or round-trip time for responses)
	meta := bytes.Split(header, []byte(" "))
	// For each request you should receive 3 payloads (request, response, replayed response) with same request id
	reqID := string(meta[1])
	payload := buf[headerSize:]

	switch payloadType {
	case requestType:
		if time.Since(lastCheck) > time.Second {
			reqsCheckedPerSeq = 0
			lastCheck = time.Now()

			// Print stats every second
			_, _ = os.Stderr.WriteString(fmt.Sprintf(
				"middleware stats: pendingRequests=%d requestsAdded=%d ok=%d diffs=%d ignored=%d\n",
				len(pendingRequests),
				pendingRequestsAdded,
				okCount,
				diffsCount,
				ignoredCount,
			))
		}

		if reqsCheckedPerSeq < maxPerSecond {
			pendingRequests[reqID] = &Request{
				Headers: payload,
			}
			pendingRequestsAdded++
			reqsCheckedPerSeq++
		}

		// Emitting data back, without modification
		_, err := io.WriteString(stdout, hex.EncodeToString(buf)+"\n")
		if err != nil {
			_, _ = io.WriteString(stderr, fmt.Sprintf("stdout.WriteString error: %v", err))
		}
	case originalResponseType:
		if req, ok := pendingRequests[reqID]; ok {
			// Original response can arrive after mirrored so this should be improved
			// instead of ignoring this case.
			req.OriginalResponse = payload
		}
	case replayedResponseType:
		if req, ok := pendingRequests[reqID]; ok {
			req.MirroredResponse = payload

			if req.IsIgnored() {
				ignoredCount++
			} else {
				if !req.ResponseEquals() {
					// TODO in the future publish the results to S3 for easier processing
					log.WithFields(log.F{
						"expected": req.OriginalBody(),
						"actual":   req.MirroredBody(),
						"headers":  string(req.Headers),
						"path":     string(proto.Path(req.Headers)),
					}).Info("Mismatch found")
					diffsCount++
				} else {
					okCount++
				}
			}

			delete(pendingRequests, reqID)
		}
	default:
		_, _ = io.WriteString(stderr, "Unknown message type\n")
	}
}
