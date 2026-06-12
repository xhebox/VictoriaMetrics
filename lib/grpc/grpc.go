package grpc

import (
	"encoding/binary"
	"fmt"
	"net/http"
)

// https://github.com/grpc/grpc/blob/master/doc/statuscodes.md
const (
	StatusCodeOk                 = "0"
	StatusCodeCancelled          = "1"
	StatusCodeUnknown            = "2"
	StatusCodeInvalidArgument    = "3"
	StatusCodeDeadlineExceeded   = "4"
	StatusCodeNotFound           = "5"
	StatusCodeAlreadyExists      = "6"
	StatusCodePermissionDenied   = "7"
	StatusCodeResourceExhausted  = "8"
	StatusCodeFailedPrecondition = "9"
	StatusCodeAborted            = "10"
	StatusCodeOutOfRange         = "11"
	StatusCodeUnimplemented      = "12"
	StatusCodeInternal           = "13"
	StatusCodeUnavailable        = "14"
	StatusCodeDataLoss           = "15"
	StatusCodeUnauthenticated    = "16"
)

const MessageHeaderSize = 5

// ParseMessageHeader parses the gRPC message envelope header.
//
// The gRPC message envelope header contains a 1-byte compressed flag followed
// by a 4-byte big-endian message length.
func ParseMessageHeader(header []byte) (messageLength uint32, compressed bool, err error) {
	if len(header) != MessageHeaderSize {
		return 0, false, fmt.Errorf("invalid gRPC header length: %d", len(header))
	}
	compressedFlag := header[0]
	if compressedFlag != 0 && compressedFlag != 1 {
		return 0, false, fmt.Errorf("invalid gRPC compressed flag %d", compressedFlag)
	}
	messageLength = binary.BigEndian.Uint32(header[1:MessageHeaderSize])
	return messageLength, compressedFlag == 1, nil
}

// ParseMessageFrame verifies and strips the gRPC message envelope.
//
// The gRPC message envelope contains a 1-byte compressed flag, a 4-byte
// big-endian message length, and the protobuf message bytes.
// See https://grpc.github.io/grpc/core/md_doc__p_r_o_t_o_c_o_l-_h_t_t_p2.html
func ParseMessageFrame(frame []byte) (message []byte, compressed bool, err error) {
	n := len(frame)
	if n < MessageHeaderSize {
		return nil, false, fmt.Errorf("invalid gRPC header length: %d", n)
	}
	messageLength, compressed, err := ParseMessageHeader(frame[:MessageHeaderSize])
	if err != nil {
		return nil, false, err
	}
	if uint64(n) != uint64(MessageHeaderSize)+uint64(messageLength) {
		return nil, false, fmt.Errorf("invalid gRPC message length: %d, actual length: %d", messageLength, n-MessageHeaderSize)
	}
	return frame[MessageHeaderSize:], compressed, nil
}

// AppendMessageFrame appends the gRPC message envelope with the given message to dst.
func AppendMessageFrame(dst, message []byte, compressed bool) []byte {
	start := len(dst)
	dst = append(dst, 0, 0, 0, 0, 0)
	if compressed {
		dst[start] = 1
	}
	binary.BigEndian.PutUint32(dst[start+1:start+MessageHeaderSize], uint32(len(message)))
	return append(dst, message...)
}

// WriteResponse writes a successful gRPC response with the given protobuf message body.
func WriteResponse(w http.ResponseWriter, body []byte) (int, error) {
	h := w.Header()
	h.Set("content-type", "application/grpc+proto")
	h.Set("trailer", "grpc-status")
	n, err := w.Write(AppendMessageFrame(nil, body, false))
	h.Set("grpc-status", StatusCodeOk)
	return n, err
}

// WriteErrorGrpcResponse writes an error response in gRPC protocol over HTTP.
func WriteErrorGrpcResponse(w http.ResponseWriter, grpcErrorCode, grpcErrorMessage string) {
	h := w.Header()
	h.Set("content-type", "application/grpc+proto")
	h.Set("grpc-status", grpcErrorCode)
	h.Set("grpc-message", encodeStatusMessage(grpcErrorMessage))
}

func encodeStatusMessage(s string) string {
	var dst []byte
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 0x20 && c < 0x7f && c != '%' {
			if dst != nil {
				dst = append(dst, c)
			}
			continue
		}
		if dst == nil {
			dst = append([]byte{}, s[:i]...)
		}
		dst = append(dst, '%', upperhex[c>>4], upperhex[c&15])
	}
	if dst == nil {
		return s
	}
	return string(dst)
}

const upperhex = "0123456789ABCDEF"
