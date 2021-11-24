package log

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"time"

	"github.com/Abdirahman0022/airline/infrastructure/util"
)

type traceDataType int

const traceDataKey traceDataType = 1

type logModel struct {
	Severity string `json:"severity"`
	Message  string `json:"message"`
	Location string `json:"location"`
	Time     string `json:"time"`
}

func newLogModel(flag, loc string, msg, trid interface{}) string {

	if flag == "ERROR" {
		return util.MustJSON(logModel{
			Severity: flag,
			Message:  fmt.Sprintf("%v %v %v", trid, loc, msg),
			Location: loc,
			Time:     time.Now().String(),
		})
	}

	return util.MustJSON(logModel{
		Severity: flag,
		Message:  fmt.Sprintf("%v %v", trid, msg),
		Location: loc,
		Time:     time.Now().String(),
	})
}

// logPrinterDefault is default implementation of LogPrinter
type logPrinterDefault struct {
}

// WriteContext passing data to
func (r *logPrinterDefault) WriteContext(ctx context.Context, traceID string) context.Context {
	return context.WithValue(ctx, traceDataKey, traceID)
}

// LogPrint simply print the message to console
func (r *logPrinterDefault) LogPrint(ctx context.Context, flag string, data interface{}) {

	// default traceId
	traceID := "0000000000000000"

	if ctx != nil {
		if v := ctx.Value(traceDataKey); v != nil {
			traceID = v.(string)
		}
	}

	fmt.Println(newLogModel(flag, GetFileLocationInfo(3), data, traceID))
}

// GetFileLocationInfo get the function information like filename and line number
// skip is the parameter that need to adjust if we add new method layer
func GetFileLocationInfo(skip int) string {
	pc, _, line, ok := runtime.Caller(skip)
	if !ok {
		return ""
	}
	funcName := runtime.FuncForPC(pc).Name()
	x := strings.LastIndex(funcName, "/")
	return fmt.Sprintf("%s:%d", funcName[x+1:], line)
}
