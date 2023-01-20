package logger

import (
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"go-ooo/config"
	"os"
)

func init() {
	log.SetFormatter(&log.TextFormatter{
		FullTimestamp: true,
	})

	logLevel := viper.GetString(config.LogLevel)
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
	log.SetOutput(os.Stdout)
}

type Fields map[string]interface{}

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
