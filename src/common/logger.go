package common

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

var (
	MainLogger    zerolog.Logger
	DebugLogger   *LoggerDebug
	InfoLogger    *LoggerInfo
	WarningLogger *LoggerWarn
	ErrorLogger   *LoggerError
)

func init() {
	output := zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: time.RFC3339}
	MainLogger = zerolog.New(output).With().Timestamp().Caller().Logger()
}

type LoggerDebug struct{}

func (l *LoggerDebug) Println(v ...any) {
	ev := MainLogger.Debug().CallerSkipFrame(1)
	for i, val := range v {
		ev.Any(fmt.Sprintf("val_%d", i), val)
	}
	ev.Msg("")
}

func (l *LoggerDebug) Printf(templ string, v ...any) {
	templ = strings.TrimSuffix(templ, "\n")
	MainLogger.Debug().CallerSkipFrame(1).Msgf(templ, v...)
}

type LoggerInfo struct{}

func (l *LoggerInfo) Println(v ...any) {
	ev := MainLogger.Info().CallerSkipFrame(1)
	for i, val := range v {
		ev.Any(fmt.Sprintf("val_%d", i), val)
	}
	ev.Msg("")
}

func (l *LoggerInfo) Printf(templ string, v ...any) {
	templ = strings.TrimSuffix(templ, "\n")
	MainLogger.Info().CallerSkipFrame(1).Msgf(templ, v...)
}

type LoggerWarn struct{}

func (l *LoggerWarn) Println(v ...any) {
	ev := MainLogger.Warn().CallerSkipFrame(1)
	for i, val := range v {
		ev.Any(fmt.Sprintf("val_%d", i), val)
	}
	ev.Msg("")
}

func (l *LoggerWarn) Printf(templ string, v ...any) {
	templ = strings.TrimSuffix(templ, "\n")
	MainLogger.Warn().CallerSkipFrame(1).Msgf(templ, v...)
}

type LoggerError struct{}

func (l *LoggerError) Println(v ...any) {
	ev := MainLogger.Error().CallerSkipFrame(1)
	for i, val := range v {
		ev.Any(fmt.Sprintf("val_%d", i), val)
	}
	ev.Msg("")
}

func (l *LoggerError) Printf(templ string, v ...any) {
	templ = strings.TrimSuffix(templ, "\n")
	MainLogger.Error().CallerSkipFrame(1).Msgf(templ, v...)
}

func (l *LoggerError) Fatal(v ...any) {
	ev := MainLogger.Fatal().CallerSkipFrame(1)
	for i, val := range v {
		ev.Any(fmt.Sprintf("val_%d", i), val)
	}
	ev.Msg("")
}

func (l *LoggerError) Fatalln(v ...any) {
	ev := MainLogger.Fatal().CallerSkipFrame(1)
	for i, val := range v {
		ev.Any(fmt.Sprintf("val_%d", i), val)
	}
	ev.Msg("")
}
