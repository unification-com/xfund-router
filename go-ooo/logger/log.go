package logger

import (
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"go-ooo/config"
	"os"
)

type Logger struct {
	logger *logrus.Logger
}

type Fields map[string]interface{}

func NewAppLogger() *Logger {
	return &Logger{logger: logrus.New()}
}

func packFields(pkg, function, action string, fields Fields) logrus.Fields {
	packedFields := logrus.Fields{
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

func (l *Logger) InitLogger() {
	l.logger.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})

	logLevel := viper.GetString(config.LogLevel)
	logrusLevel := logrus.InfoLevel

	switch logLevel {
	case "info":
		logrusLevel = logrus.InfoLevel
		break
	case "debug":
		logrusLevel = logrus.DebugLevel
		break
	default:
		logrusLevel = logrus.InfoLevel
		break
	}

	l.logger.SetLevel(logrusLevel)
	l.logger.SetOutput(os.Stdout)
}

func (l *Logger) Info(pkg, function, action, msg string) {

	packedFields := packFields(pkg, function, action, nil)

	l.logger.WithFields(packedFields).Info(msg)
}

func (l *Logger) InfoWithFields(pkg, function, action, msg string, fields Fields) {

	packedFields := packFields(pkg, function, action, fields)

	l.logger.WithFields(packedFields).Info(msg)
}

func (l *Logger) Debug(pkg, function, action, msg string, fields Fields) {

	packedFields := packFields(pkg, function, action, fields)

	l.logger.WithFields(packedFields).Debug(msg)
}

func (l *Logger) Warn(pkg, function, action, msg string) {

	packedFields := packFields(pkg, function, action, nil)

	l.logger.WithFields(packedFields).Warn(msg)
}

func (l *Logger) WarnWithFields(pkg, function, action, msg string, fields Fields) {

	packedFields := packFields(pkg, function, action, fields)

	l.logger.WithFields(packedFields).Warn(msg)
}

func (l *Logger) Error(pkg, function, action, msg string) {

	packedFields := packFields(pkg, function, action, nil)

	l.logger.WithFields(packedFields).Error(msg)
}

func (l *Logger) ErrorWithFields(pkg, function, action, msg string, fields Fields) {

	packedFields := packFields(pkg, function, action, fields)

	l.logger.WithFields(packedFields).Error(msg)
}

func (l *Logger) Panic(pkg, function, action, msg string) {

	packedFields := packFields(pkg, function, action, nil)

	l.logger.WithFields(packedFields).Panic(msg)
}

func (l *Logger) PanicWithFields(pkg, function, action, msg string, fields Fields) {

	packedFields := packFields(pkg, function, action, fields)

	l.logger.WithFields(packedFields).Panic(msg)
}

func (l *Logger) Fatal(pkg, function, action, msg string) {

	packedFields := packFields(pkg, function, action, nil)

	l.logger.WithFields(packedFields).Fatal(msg)
}

func (l *Logger) FatalWithFields(pkg, function, action, msg string, fields Fields) {

	packedFields := packFields(pkg, function, action, fields)

	l.logger.WithFields(packedFields).Fatal(msg)
}
