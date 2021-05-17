package ecslog

import (
	"bufio"
	"github.com/trentm/go-ecslog/internal/jsonutils"
	"github.com/valyala/fastjson"
	"io"
	"sort"
	"time"
)

type parallelReader struct {
	readers map[string]*bufio.Reader
	parser  fastjson.Parser
	buffer  Records
}

type Record struct {
	Source    string
	Timestamp time.Time
	Data      *fastjson.Value
}

type Records []Record

func (a Records) Len() int           { return len(a) }
func (a Records) Less(i, j int) bool { return a[i].Timestamp.Before(a[j].Timestamp) }
func (a Records) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }

// NewParallelReader reads data from multiple streams sorted by timestamp
func NewParallelReader(inputs map[string]io.Reader, maxLineLen int, p fastjson.Parser) *parallelReader {
	const minBufSize = 65536
	bufSize := maxLineLen + 2
	if bufSize < minBufSize {
		bufSize = minBufSize
	}

	pr := &parallelReader{parser: p, readers: make(map[string]*bufio.Reader)}
	for name, in := range inputs {
		pr.readers[name] = bufio.NewReaderSize(in, bufSize)
	}
	pr.readFirstRecords()
	return pr
}

//func (pr *parallelReader) Read(p []byte) (n int, err error) {
//	return
//}

func (pr *parallelReader) readFirstRecords() {
	for k, v := range pr.readers {
		rec := readline(k, v, pr.parser)
		if rec != nil {
			pr.buffer = append(pr.buffer, *rec)
		}
	}
	sort.Sort(pr.buffer)
}

func (pr *parallelReader) ReadNextRecord() *Record {
	if len(pr.buffer) == 0 {
		return nil
	}
	rec := pr.buffer[0]
	pr.buffer = pr.buffer[1:]
	reader, ok := pr.readers[rec.Source]
	if !ok {
		return &rec
	}
	next := readline(rec.Source, reader, pr.parser)
	if next != nil {
		pr.buffer = append(pr.buffer, *next)
		sort.Sort(pr.buffer)
	}
	return &rec
}

func readline(src string, r *bufio.Reader, p fastjson.Parser) *Record {
	line, isPrefix, err := r.ReadLine()
	// TODO handle isPrefix, errors, EOF, etc in the same way that is done before this PR: long lines and not ECS lines
	// should be returned as is.
	// Record probably should hold raw data
	if err == nil && !isPrefix {
		b, _ := p.ParseBytes(line)
		timestamp := jsonutils.LookupValue(b, "@timestamp").GetStringBytes()
		t, _ := time.Parse("2006-01-02T15:04:05.999999999-0700", string(timestamp))
		return &Record{src, t, b}
	}
	return nil
}
