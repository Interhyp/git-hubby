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
