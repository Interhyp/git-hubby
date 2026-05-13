/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/zap/zapcore"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var _ = Describe("parseLogLevel", func() {
	DescribeTable("parses log level strings correctly",
		func(input string, expectedLevel zapcore.Level, expectedOk bool) {
			level, ok := parseLogLevel(input)
			Expect(ok).To(Equal(expectedOk))
			Expect(level).To(Equal(expectedLevel))
		},
		Entry("empty string returns false", "", zapcore.InfoLevel, false),
		Entry("whitespace only returns false", "   ", zapcore.InfoLevel, false),
		Entry("debug level", "debug", zapcore.DebugLevel, true),
		Entry("info level", "info", zapcore.InfoLevel, true),
		Entry("warn level", "warn", zapcore.WarnLevel, true),
		Entry("error level", "error", zapcore.ErrorLevel, true),
		Entry("uppercase DEBUG is accepted", "DEBUG", zapcore.DebugLevel, true),
		Entry("mixed case Info is accepted", "Info", zapcore.InfoLevel, true),
		Entry("invalid value returns false", "verbose", zapcore.InfoLevel, false),
		Entry("numeric value is invalid", "2", zapcore.InfoLevel, false),
		Entry("value with surrounding whitespace is trimmed", "  warn  ", zapcore.WarnLevel, true),
	)
})

var _ = Describe("parseLogFormat", func() {
	DescribeTable("resolves log format correctly",
		func(input string, expected logFormat) {
			Expect(parseLogFormat(input)).To(Equal(expected))
		},
		Entry("lowercase ecs", "ecs", logFormatECS),
		Entry("uppercase ECS", "ECS", logFormatECS),
		Entry("mixed case Ecs", "Ecs", logFormatECS),
		Entry("ecs with whitespace", "  ecs  ", logFormatECS),
		Entry("lowercase console", "console", logFormatConsole),
		Entry("uppercase CONSOLE", "CONSOLE", logFormatConsole),
		Entry("mixed case Console", "Console", logFormatConsole),
		Entry("console with whitespace", "  console  ", logFormatConsole),
		Entry("lowercase json defaults to json", "json", logFormatJSON),
		Entry("uppercase JSON defaults to json", "JSON", logFormatJSON),
		Entry("empty string defaults to json", "", logFormatJSON),
		Entry("whitespace only defaults to json", "   ", logFormatJSON),
		Entry("unknown value defaults to json", "foobar", logFormatJSON),
		Entry("text defaults to json", "text", logFormatJSON),
	)
})

var _ = Describe("buildLoggerOpts", func() {
	It("returns opts with standard JSON encoder when format is json", func() {
		flagOpts := &zap.Options{Development: true}
		result := buildLoggerOpts(flagOpts, logFormatJSON)
		// UseFlagOptions + RawZapOpts(LogMapper) + JSONEncoder = 3 opts
		Expect(result).To(HaveLen(3))
	})

	It("returns opts with ECS encoder when format is ecs", func() {
		flagOpts := &zap.Options{Development: true}
		result := buildLoggerOpts(flagOpts, logFormatECS)
		// UseFlagOptions + RawZapOpts(LogMapper) + JSONEncoder(ecs) + RawZapOpts(ecsWrapCore) = 4 opts
		Expect(result).To(HaveLen(4))
	})

	It("returns opts with console encoder when format is console", func() {
		flagOpts := &zap.Options{Development: true}
		result := buildLoggerOpts(flagOpts, logFormatConsole)
		// UseFlagOptions + RawZapOpts(LogMapper) + ConsoleEncoder = 3 opts
		Expect(result).To(HaveLen(3))
	})
})
