package ledgerbackend

import (
	"bufio"
	"io"
	"regexp"
	"strings"
	"sync"

	"github.com/stellar/go-stellar-sdk/support/log"
)

type logLineWriter struct {
	pipeReader *io.PipeReader
	pipeWriter *io.PipeWriter
	wg         sync.WaitGroup
	log        *log.Entry
}

func newLogLineWriter(log *log.Entry) *logLineWriter {
	rd, wr := io.Pipe()
	return &logLineWriter{
		pipeReader: rd,
		pipeWriter: wr,
		log:        log,
	}
}

func (l *logLineWriter) Write(p []byte) (n int, err error) {
	return l.pipeWriter.Write(p)
}

func (l *logLineWriter) Close() error {
	err := l.pipeWriter.Close()
	l.wg.Wait()
	return err
}

func (l *logLineWriter) Start() {
	br := bufio.NewReader(l.pipeReader)
	l.wg.Add(1)
	go func() {
		defer l.wg.Done()
		dateRx := regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d{3} `)
		levelRx := regexp.MustCompile(`\[(\w+) ([A-Z]+)\] (.*)`)
		for {
			line, err := br.ReadString('\n')
			if err != nil {
				break
			}
			line = dateRx.ReplaceAllString(line, "")
			line = strings.TrimSpace(line)

			if line == "" {
				continue
			}

			matches := levelRx.FindStringSubmatch(line)
			if len(matches) >= 4 {
				// Extract the substrings from the log entry and trim it
				category, level := matches[1], matches[2]
				line = matches[3]

				levelMapping := map[string]func(string, ...interface{}){
					"FATAL":   l.log.Errorf,
					"ERROR":   l.log.Errorf,
					"WARNING": l.log.Warnf,
					"INFO":    l.log.Infof,
					"DEBUG":   l.log.Debugf,
				}

				writer := l.log.Infof
				if f, ok := levelMapping[strings.ToUpper(level)]; ok {
					writer = f
				}
				writer("%s: %s", category, line)
			} else {
				l.log.Info(line)
			}
		}
	}()
}
