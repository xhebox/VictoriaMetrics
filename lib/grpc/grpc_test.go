package grpc

import (
	"bytes"
	"encoding/binary"
	"io"
	"net/http/httptest"
	"testing"
)

func TestParseMessageFrame(t *testing.T) {
	message := []byte("foobar")
	frame := AppendMessageFrame(nil, message, false)
	gotMessage, gotCompressed, err := ParseMessageFrame(frame)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if gotCompressed {
		t.Fatalf("unexpected compressed flag; got true; want false")
	}
	if !bytes.Equal(gotMessage, message) {
		t.Fatalf("unexpected message; got %q; want %q", gotMessage, message)
	}

	frame = AppendMessageFrame(frame[:0], message, true)
	gotMessage, gotCompressed, err = ParseMessageFrame(frame)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if !gotCompressed {
		t.Fatalf("unexpected compressed flag; got false; want true")
	}
	if !bytes.Equal(gotMessage, message) {
		t.Fatalf("unexpected message; got %q; want %q", gotMessage, message)
	}
}
func TestParseMessageHeader(t *testing.T) {
	message := []byte("foobar")
	frame := AppendMessageFrame(nil, message, true)
	messageLength, compressed, err := ParseMessageHeader(frame[:MessageHeaderSize])
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if got, want := messageLength, uint32(len(message)); got != want {
		t.Fatalf("unexpected message length; got %d; want %d", got, want)
	}
	if !compressed {
		t.Fatalf("unexpected compressed flag; got false; want true")
	}

	if _, _, err := ParseMessageHeader(frame[:MessageHeaderSize-1]); err == nil {
		t.Fatalf("expecting non-nil error for short header")
	}
	frame[0] = 2
	if _, _, err := ParseMessageHeader(frame[:MessageHeaderSize]); err == nil {
		t.Fatalf("expecting non-nil error for invalid compressed flag")
	}
}

func TestParseMessageFrameFailure(t *testing.T) {
	f := func(name string, frame []byte) {
		t.Helper()
		if _, _, err := ParseMessageFrame(frame); err == nil {
			t.Fatalf("%s: expecting non-nil error", name)
		}
	}

	f("short", []byte{0, 0, 0, 0})

	invalidFlag := AppendMessageFrame(nil, []byte("foo"), false)
	invalidFlag[0] = 2
	f("invalid compressed flag", invalidFlag)

	invalidLen := AppendMessageFrame(nil, []byte("foo"), false)
	binary.BigEndian.PutUint32(invalidLen[1:5], 4)
	_, _, err := ParseMessageFrame(invalidLen)
	if err == nil {
		t.Fatalf("invalid message length: expecting non-nil error")
	}
	if got, want := err.Error(), "invalid gRPC message length: 4, actual length: 3"; got != want {
		t.Fatalf("unexpected error; got %q; want %q", got, want)
	}

	tooBigLen := AppendMessageFrame(nil, []byte("foo"), false)
	binary.BigEndian.PutUint32(tooBigLen[1:5], ^uint32(0))
	f("too big declared message length", tooBigLen)
}

func TestWriteResponse(t *testing.T) {
	rr := httptest.NewRecorder()
	n, err := WriteResponse(rr, nil)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if n != 5 {
		t.Fatalf("unexpected number of response bytes written; got %d; want 5", n)
	}
	resp := rr.Result()
	defer resp.Body.Close()
	if got, want := resp.Header.Get("content-type"), "application/grpc+proto"; got != want {
		t.Fatalf("unexpected content-type; got %q; want %q", got, want)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("cannot read response body: %s", err)
	}
	message, compressed, err := ParseMessageFrame(body)
	if err != nil {
		t.Fatalf("cannot parse response body: %s", err)
	}
	if compressed {
		t.Fatalf("unexpected compressed response")
	}
	if len(message) != 0 {
		t.Fatalf("unexpected response message; got %q; want empty", message)
	}
	if got, want := resp.Trailer.Get("grpc-status"), StatusCodeOk; got != want {
		t.Fatalf("unexpected grpc-status trailer; got %q; want %q", got, want)
	}
}

func TestWriteErrorGrpcResponse(t *testing.T) {
	rr := httptest.NewRecorder()
	WriteErrorGrpcResponse(rr, StatusCodeInternal, "cannot process: 100% busted\n")
	resp := rr.Result()
	defer resp.Body.Close()
	if got, want := resp.Header.Get("content-type"), "application/grpc+proto"; got != want {
		t.Fatalf("unexpected content-type; got %q; want %q", got, want)
	}
	if got, want := resp.Header.Get("grpc-status"), StatusCodeInternal; got != want {
		t.Fatalf("unexpected grpc-status; got %q; want %q", got, want)
	}
	if got, want := resp.Header.Get("grpc-message"), "cannot process: 100%25 busted%0A"; got != want {
		t.Fatalf("unexpected grpc-message; got %q; want %q", got, want)
	}
}
