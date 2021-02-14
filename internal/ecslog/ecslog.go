package ecslog

import (
	"strings"

	"github.com/valyala/fastjson"
	"go.uber.org/zap"
)

// Shared stuff for `ecslog` that isn't specific to the CLI.

// State stores ECS log record processing state.
type State struct {
	Log       *zap.Logger // singleton internal logger for an `ecslog` run
	LogLevel  string      // extracted "log.level" for the current record
	Timestamp []byte      // extracted "@timestamp" for the current record
	Message   []byte      // extracted "message" for the current record
}

// NewState returns a new State object for holding processing state.
func NewState(logger *zap.Logger) *State {
	return &State{Log: logger}
}

// levelValFromName is a best-effort ordering of levels in common usage in
// logging frameworks that might be used in ECS format. See `ECSLevelLess`
// below.
var levelValFromName = map[string]int{
	"trace":   10,
	"debug":   20,
	"info":    30,
	"warn":    40,
	"warning": 40,
	"error":   50,
	"fatal":   60,
}

// ECSLevelLess returns true iff level1 is less than level2.
//
// Because ECS doesn't mandate a set of log level names for the "log.level"
// field, nor any specific ordering of those log levels, this is a best
// effort based on names and ordering from common logging frameworks.
// If a level name is unknown, this returns false. Level names are considered
// case-insensitive.
func ECSLevelLess(level1, level2 string) bool {
	val1, ok := levelValFromName[strings.ToLower(level1)]
	if !ok {
		return false
	}
	val2, ok := levelValFromName[strings.ToLower(level2)]
	if !ok {
		return false
	}
	return val1 < val2
}

// DottedGetBytes looks up key "$aStr.$bStr" in the given record and removes
// those entries from the record.
func DottedGetBytes(rec *fastjson.Value, aStr, bStr string) []byte {
	var abBytes []byte

	// Try `{"a": {"b": <value>}}`.
	aObj := rec.GetObject(aStr)
	if aObj != nil {
		abVal := aObj.Get(bStr)
		if abVal != nil {
			abBytes = abVal.GetStringBytes()
			aObj.Del(bStr)
			if aObj.Len() == 0 {
				rec.Del(aStr)
			}
		}
	}

	// Try `{"a.b": <value>}`.
	if abBytes == nil {
		abStr := aStr + "." + bStr
		abBytes = rec.GetStringBytes(abStr)
		if abBytes != nil {
			rec.Del(abStr)
		}
	}

	return abBytes
}

// IsECSLoggingRecord returns true iff the given `rec` has the required
// ecs-logging fields.
//
// It also *mutates* the given `st` runtime state and `rec` record: populating
// `st` with the extracted core fields, while deleting those fields from `rec`.
// This is for performance, to avoid having to lookup those fields twice.
func IsECSLoggingRecord(st *State, rec *fastjson.Value) bool {
	timestamp := rec.GetStringBytes("@timestamp")
	if timestamp == nil {
		return false
	}
	st.Timestamp = timestamp
	rec.Del("@timestamp")

	message := rec.GetStringBytes("message")
	if message == nil {
		return false
	}
	st.Message = message
	rec.Del("message")

	ecsVersion := DottedGetBytes(rec, "ecs", "version")
	if ecsVersion == nil {
		return false
	}

	logLevel := DottedGetBytes(rec, "log", "level")
	if logLevel == nil {
		return false
	}
	st.LogLevel = string(logLevel)

	return true
}
