package log_v1

import (
	"fmt"
	"net/http"

	"google.golang.org/genproto/googleapis/rpc/errdetails"
	status "google.golang.org/grpc/status"
)

type ErrOffsetOutOfRange struct {
	Offset uint64
}

func (e ErrOffsetOutOfRange) GRPCStatus() *status.Status {
	st := status.New(
		http.StatusNotFound,
		fmt.Sprintf("offset out of range: %d", e.Offset),
	)

	msg := fmt.Sprintf(
		"The requested offset is outside the log's range: %d",
		e.Offset,
	)

	d := &errdetails.LocalizedMessage{
		Locale:  "en-GB",
		Message: msg,
	}
	std, err := st.WithDetails(d)
	if err != nil {
		return st
	}

	return std
}

func (e ErrOffsetOutOfRange) Error() string {
	return e.GRPCStatus().Err().Error()
}
