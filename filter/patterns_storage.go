package filter

import (
	"fmt"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/aristanetworks/goarista/monotime"
	"github.com/segmentio/fasthash/fnv1a"

	"go.avito.ru/DO/moira"
	"go.avito.ru/DO/moira/metrics"
)

const (
	bufferAlloc   = 256
	heartBeatCap  = 10000
	heartBeatDrop = 8000

	processThreshold = 10 * time.Millisecond
)

var (
	asteriskHash = fnv1a.HashString64("*")
)

// PatternStorage contains pattern tree
type PatternStorage struct {
	database moira.Database
	logger   moira.Logger
	metrics  *metrics.FilterMetrics

	heartbeat   chan bool
	matcherPool sync.Pool
	PatternTree *patternNode
}

// matcherBuffer is operative buffer for PatternStorage.matchPattern method
// it helps to avoid redundant allocations
type matcherBuffer struct {
	curr []*patternNode // curr is tree's nodes (horizontal) level which is being currently processed
	next []*patternNode // next is tree's nodes (horizontal) level which is supposed to be processed after current
}

// patternNode contains pattern node
type patternNode struct {
	Children   []*patternNode
	Part       string
	Hash       uint64
	Prefix     string
	InnerParts []string
}

// NewPatternStorage creates new PatternStorage struct
func NewPatternStorage(
	database moira.Database,
	metrics *metrics.FilterMetrics,
	logger moira.Logger,
) (*PatternStorage, error) {
	storage := &PatternStorage{
		database:  database,
		logger:    logger,
		metrics:   metrics,
		heartbeat: make(chan bool, heartBeatCap),
	}
	storage.matcherPool = sync.Pool{
		New: func() interface{} {
			return &matcherBuffer{
				curr: make([]*patternNode, 0, bufferAlloc),
				next: make([]*patternNode, 0, bufferAlloc),
			}
		},
	}
	err := storage.RefreshTree()
	return storage, err
}

// GetHeartbeat returns heartbeat chan
func (storage *PatternStorage) GetHeartbeat() chan bool {
	return storage.heartbeat
}

// ProcessIncomingMetric validates, parses and matches incoming raw string
func (storage *PatternStorage) ProcessIncomingMetric(line []byte) *moira.MatchedMetric {
	if len(storage.heartbeat) < heartBeatDrop {
		storage.heartbeat <- true
	}
	storage.metrics.TotalMetricsReceived.Increment()

	metric, value, timestamp, err := parseMetricFromString(line)
	if err != nil {
		storage.logger.InfoF("cannot parse input: %v", err)
		return nil
	}
	storage.metrics.ValidMetricsReceived.Increment()

	matchingStart := monotime.Now()
	matched := storage.matchPattern(metric)
	matchingDuration := monotime.Since(matchingStart)

	storage.metrics.MatchingTimer.Timing(int64(matchingDuration))
	if matchingDuration > processThreshold {
		storage.logger.WarnF("[Attempt 2] It took too long (%s) to process metric: %s", matchingDuration.String(), line)
	}

	if len(matched) > 0 {
		storage.metrics.MatchingMetricsReceived.Increment()
		return &moira.MatchedMetric{
			Metric:             metric,
			Patterns:           matched,
			Value:              value,
			Timestamp:          timestamp,
			RetentionTimestamp: timestamp,
			Retention:          60,
		}
	}

	return nil
}

// RefreshTree builds pattern tree from redis data
func (storage *PatternStorage) RefreshTree() error {
	patterns, err := storage.database.GetPatterns()
	if err != nil {
		return err
	}

	return storage.buildTree(patterns)
}

// matchPattern returns array of matched patterns
func (storage *PatternStorage) matchPattern(metric string) []string {
	buff := storage.matchedBufferAcquire()
	defer storage.matchedBufferRelease(buff)

	index := 0
	for i, c := range metric {
		if c == '.' {
			part := metric[index:i]
			if len(part) == 0 {
				return []string{}
			}
			index = i + 1

			if !buff.findPart(part) {
				return []string{}
			}
		}
	}

	part := metric[index:]
	if !buff.findPart(part) {
		return []string{}
	}

	matched := make([]string, 0, len(buff.curr))
	for _, node := range buff.curr {
		if len(node.Children) == 0 {
			matched = append(matched, node.Prefix)
		}
	}

	return matched
}

func (storage *PatternStorage) buildTree(patterns []string) error {
	newTree := &patternNode{}

	for _, pattern := range patterns {
		currentNode := newTree
		parts := strings.Split(pattern, ".")
		if hasEmptyParts(parts) {
			continue
		}

		for _, part := range parts {
			found := false
			for _, child := range currentNode.Children {
				if part == child.Part {
					currentNode = child
					found = true
					break
				}
			}

			if !found {
				newNode := &patternNode{Part: part}

				if currentNode.Prefix == "" {
					newNode.Prefix = part
				} else {
					newNode.Prefix = fmt.Sprintf("%s.%s", currentNode.Prefix, part)
				}

				if part == "*" || !strings.ContainsAny(part, "{*?") {
					newNode.Hash = fnv1a.HashString64(part)
				} else if strings.Contains(part, "{") && strings.Contains(part, "}") {
					prefix, bigSuffix := split2(part, "{")
					inner, suffix := split2(bigSuffix, "}")
					innerParts := strings.Split(inner, ",")

					newNode.InnerParts = make([]string, 0, len(innerParts))
					for _, innerPart := range innerParts {
						newNode.InnerParts = append(newNode.InnerParts, fmt.Sprintf("%s%s%s", prefix, innerPart, suffix))
					}
				} else {
					newNode.InnerParts = []string{part}
				}

				currentNode.Children = append(currentNode.Children, newNode)
				currentNode = newNode
			}
		}
	}

	storage.PatternTree = newTree
	return nil
}

// matchedBufferAcquire acquires buffer from pool and initializes it with tree's root
func (storage *PatternStorage) matchedBufferAcquire() *matcherBuffer {
	buff := storage.matcherPool.Get().(*matcherBuffer)
	buff.curr = append(buff.curr, storage.PatternTree)
	return buff
}

// matchedBufferRelease returns buffer to pool as well as clears its nodes levels
func (storage *PatternStorage) matchedBufferRelease(buff *matcherBuffer) {
	buff.curr = buff.curr[:0]
	buff.next = buff.next[:0]
	storage.matcherPool.Put(buff)
}

// findPart seeks for the given part among current nodes level
// while preparing the next level
func (buff *matcherBuffer) findPart(part string) bool {
	buff.next = buff.next[:0]

	hash := fnv1a.HashString64(part)
	match := false

	for _, node := range buff.curr {
		for _, child := range node.Children {
			match = false

			if child.Hash == asteriskHash || child.Hash == hash {
				match = true
			} else {
				for _, innerPart := range child.InnerParts {
					innerMatch, _ := path.Match(innerPart, part)
					if innerMatch {
						match = true
						break
					}
				}
			}

			if match {
				buff.next = append(buff.next, child)
			}
		}
	}

	// swap current and the next level: the next one will be used at the next iteration
	buff.curr, buff.next = buff.next, buff.curr

	// it should be len(buff.next), but curr and next have just been swapped
	return len(buff.curr) > 0
}

func hasEmptyParts(parts []string) bool {
	for _, part := range parts {
		if part == "" {
			return true
		}
	}
	return false
}

// parseMetricFromString parses metric from string
// supported format: "<metricString> <valueFloat64> <timestampInt64>"
func parseMetricFromString(line []byte) (string, float64, int64, error) {
	var parts [3][]byte
	partIndex := 0
	partOffset := 0
	for i, b := range line {
		r := rune(b)
		if r > unicode.MaxASCII || !strconv.IsPrint(r) {
			return "", 0, 0, fmt.Errorf("non-ascii or non-printable chars in metric name: '%s'", line)
		}
		if b == ' ' {
			parts[partIndex] = line[partOffset:i]
			partOffset = i + 1
			partIndex++
		}
		if partIndex > 2 {
			return "", 0, 0, fmt.Errorf("too many space-separated items: '%s'", line)
		}
	}

	if partIndex < 2 {
		return "", 0, 0, fmt.Errorf("too few space-separated items: '%s'", line)
	}

	parts[partIndex] = line[partOffset:]

	metric := parts[0]
	if len(metric) < 1 {
		return "", 0, 0, fmt.Errorf("metric name is empty: '%s'", line)
	}

	value, err := strconv.ParseFloat(string(parts[1]), 64)
	if err != nil {
		return "", 0, 0, fmt.Errorf("cannot parse value: '%s' (%s)", line, err)
	}

	timestamp, err := parseTimestamp(string(parts[2]))
	if err != nil || timestamp == 0 {
		return "", 0, 0, fmt.Errorf("cannot parse timestamp: '%s' (%s)", line, err)
	}

	return string(metric), value, timestamp, nil
}

func parseTimestamp(unixTimestamp string) (int64, error) {
	timestamp, err := strconv.ParseFloat(unixTimestamp, 64)
	return int64(timestamp), err
}

func split2(s, sep string) (string, string) {
	splitResult := strings.SplitN(s, sep, 2)
	if len(splitResult) < 2 {
		return splitResult[0], ""
	}
	return splitResult[0], splitResult[1]
}
