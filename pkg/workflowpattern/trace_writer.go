package workflowpattern

import "fmt"

type TraceWriter interface {
	Info(string, ...interface{})
}

type EmptyTraceWriter struct{}

func (*EmptyTraceWriter) Info(string, ...interface{}) {
}

type StdOutTraceWriter struct{}

func (*StdOutTraceWriter) Info(format string, args ...interface{}) {
	fmt.Printf(format+"\n", args...)
}
