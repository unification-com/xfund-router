package logger

import (
	log "github.com/sirupsen/logrus"
	"os"
)

func init() {
	log.SetFormatter(&log.TextFormatter{
		FullTimestamp: true,
	})

	log.SetLevel(log.InfoLevel)
	log.SetOutput(os.Stdout)
}

type Fields map[string]interface{}

func SetLogLevel(logLevel string) {
	logrusLevel := log.InfoLevel

	switch logLevel {
	case "info":
		logrusLevel = log.InfoLevel
		break
	case "debug":
		logrusLevel = log.DebugLevel
		break
	default:
		logrusLevel = log.InfoLevel
		break
	}
	log.SetLevel(logrusLevel)
}

func packFields(pkg, function, action string, fields Fields) log.Fields {
	packedFields := log.Fields{
		"pkg":  pkg,
		"func": function,
	}

	if action != "" {
		packedFields["action"] = action
	}

	if fields != nil {
		for fn, f := range fields {
			packedFields[fn] = f
		}
	}

	return packedFields
}

// Scoped is a logger that injects a fixed set of fields (e.g. the chain a worker is bound to) into
// every line it emits, so output from concurrent contexts stays attributable. Its methods mirror the
// package-level functions; the scoped fields are merged in, and a per-call field of the same name wins.
// A nil *Scoped is safe and behaves like the package-level functions (no extra fields).
type Scoped struct {
	fields Fields
}

// NewScoped returns a logger that stamps fields onto every line.
func NewScoped(fields Fields) *Scoped {
	return &Scoped{fields: fields}
}

// merge combines the scoped fields with a call's fields (the call's value wins on a key clash).
func (s *Scoped) merge(fields Fields) Fields {
	if s == nil {
		return fields
	}
	merged := Fields{}
	for k, v := range s.fields {
		merged[k] = v
	}
	for k, v := range fields {
		merged[k] = v
	}
	return merged
}

func (s *Scoped) Info(pkg, function, action, msg string) {
	InfoWithFields(pkg, function, action, msg, s.merge(nil))
}

func (s *Scoped) InfoWithFields(pkg, function, action, msg string, fields Fields) {
	InfoWithFields(pkg, function, action, msg, s.merge(fields))
}

func (s *Scoped) Debug(pkg, function, action, msg string, fields Fields) {
	Debug(pkg, function, action, msg, s.merge(fields))
}

func (s *Scoped) Warn(pkg, function, action, msg string) {
	WarnWithFields(pkg, function, action, msg, s.merge(nil))
}

func (s *Scoped) WarnWithFields(pkg, function, action, msg string, fields Fields) {
	WarnWithFields(pkg, function, action, msg, s.merge(fields))
}

func (s *Scoped) Error(pkg, function, action, msg string) {
	ErrorWithFields(pkg, function, action, msg, s.merge(nil))
}

func (s *Scoped) ErrorWithFields(pkg, function, action, msg string, fields Fields) {
	ErrorWithFields(pkg, function, action, msg, s.merge(fields))
}

func (s *Scoped) Panic(pkg, function, action, msg string) {
	PanicWithFields(pkg, function, action, msg, s.merge(nil))
}

func (s *Scoped) PanicWithFields(pkg, function, action, msg string, fields Fields) {
	PanicWithFields(pkg, function, action, msg, s.merge(fields))
}

func (s *Scoped) Fatal(pkg, function, action, msg string) {
	FatalWithFields(pkg, function, action, msg, s.merge(nil))
}

func (s *Scoped) FatalWithFields(pkg, function, action, msg string, fields Fields) {
	FatalWithFields(pkg, function, action, msg, s.merge(fields))
}

func Info(pkg, function, action, msg string) {

	packedFields := packFields(pkg, function, action, nil)

	log.WithFields(packedFields).Info(msg)
}

func InfoWithFields(pkg, function, action, msg string, fields Fields) {

	packedFields := packFields(pkg, function, action, fields)

	log.WithFields(packedFields).Info(msg)
}

func Debug(pkg, function, action, msg string, fields Fields) {

	packedFields := packFields(pkg, function, action, fields)

	log.WithFields(packedFields).Debug(msg)
}

func Warn(pkg, function, action, msg string) {

	packedFields := packFields(pkg, function, action, nil)

	log.WithFields(packedFields).Warn(msg)
}

func WarnWithFields(pkg, function, action, msg string, fields Fields) {

	packedFields := packFields(pkg, function, action, fields)

	log.WithFields(packedFields).Warn(msg)
}

func Error(pkg, function, action, msg string) {

	packedFields := packFields(pkg, function, action, nil)

	log.WithFields(packedFields).Error(msg)
}

func ErrorWithFields(pkg, function, action, msg string, fields Fields) {

	packedFields := packFields(pkg, function, action, fields)

	log.WithFields(packedFields).Error(msg)
}

func Panic(pkg, function, action, msg string) {

	packedFields := packFields(pkg, function, action, nil)

	log.WithFields(packedFields).Panic(msg)
}

func PanicWithFields(pkg, function, action, msg string, fields Fields) {

	packedFields := packFields(pkg, function, action, fields)

	log.WithFields(packedFields).Panic(msg)
}

func Fatal(pkg, function, action, msg string) {

	packedFields := packFields(pkg, function, action, nil)

	log.WithFields(packedFields).Fatal(msg)
}

func FatalWithFields(pkg, function, action, msg string, fields Fields) {

	packedFields := packFields(pkg, function, action, fields)

	log.WithFields(packedFields).Fatal(msg)
}
