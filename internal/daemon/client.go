package daemon

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"syscall"
	"time"
)

// ErrDown is wrapped in Call's error when no daemon listens on the socket: the file is missing
// or the connect is refused. Any other error means a daemon answered, or failed mid-call.
var ErrDown = errors.New("the wk daemon is not running")

// deadline bounds the dial and the whole exchange (PD-13).
const deadline = 2 * time.Second

// Client calls the daemon listening on Socket, one connection per call. Unused, like Server:
// see Server's comment.
type Client struct{ Socket string }

// Call sends op with in's fields beside it and decodes the reply's data into out; out may be
// nil. A reply of ok: false comes back as an error carrying the daemon's message as is.
func (c Client) Call(op string, in any, out any) error {
	req, err := request(op, in)
	if err != nil {
		return err
	}
	conn, err := net.DialTimeout("unix", c.Socket, deadline)
	if err != nil {
		if errors.Is(err, syscall.ENOENT) || errors.Is(err, syscall.ECONNREFUSED) {
			return fmt.Errorf("%w: %v", ErrDown, err)
		}
		return err
	}
	defer func() { _ = conn.Close() }()
	if err := conn.SetDeadline(time.Now().Add(deadline)); err != nil {
		return err
	}
	if _, err := conn.Write(append(req, '\n')); err != nil {
		return err
	}
	return decodeReply(json.NewDecoder(conn), op, out)
}

func decodeReply(dec *json.Decoder, op string, out any) error {
	var r struct {
		OK    bool            `json:"ok"`
		Data  json.RawMessage `json:"data"`
		Error string          `json:"error"`
	}
	if err := dec.Decode(&r); err != nil {
		return fmt.Errorf("wk daemon %s: %w", op, err)
	}
	if !r.OK {
		return errors.New(r.Error)
	}
	if out == nil || len(r.Data) == 0 {
		return nil
	}
	return json.Unmarshal(r.Data, out)
}

// request puts op beside in's fields; in must encode as a JSON object, or be nil.
func request(op string, in any) ([]byte, error) {
	fields := map[string]any{}
	if in != nil {
		raw, err := json.Marshal(in)
		if err != nil {
			return nil, err
		}
		if err := json.Unmarshal(raw, &fields); err != nil {
			return nil, fmt.Errorf("wk daemon %s: the request must be a JSON object: %w", op, err)
		}
	}
	fields["op"] = op
	return json.Marshal(fields)
}
