package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// FlexString decodes a JSON string or number into a string. Cronitor returns
// some fields (for example realert_interval) as either.
type FlexString string

func (f *FlexString) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	if len(b) == 0 || bytes.Equal(b, []byte("null")) {
		*f = ""
		return nil
	}
	if b[0] == '"' {
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			return err
		}
		*f = FlexString(s)
		return nil
	}
	var n json.Number
	if err := json.Unmarshal(b, &n); err != nil {
		return fmt.Errorf("expected string or number, got %s", b)
	}
	*f = FlexString(n.String())
	return nil
}

// FlexInt decodes a JSON number or numeric string into an int64.
type FlexInt int64

func (f *FlexInt) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	if len(b) == 0 || bytes.Equal(b, []byte("null")) {
		*f = 0
		return nil
	}
	if b[0] == '"' {
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			return err
		}
		if s == "" {
			*f = 0
			return nil
		}
		n, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return fmt.Errorf("expected integer, got %q", s)
		}
		*f = FlexInt(n)
		return nil
	}
	var n json.Number
	if err := json.Unmarshal(b, &n); err != nil {
		return err
	}
	i, err := n.Int64()
	if err != nil {
		fl, ferr := n.Float64()
		if ferr != nil {
			return err
		}
		i = int64(fl)
	}
	*f = FlexInt(i)
	return nil
}

// KeyList decodes a list whose items are either plain keys or objects with a
// "key" field (e.g. [{"key":"production","name":"Production"}]) into keys.
type KeyList []string

func (k *KeyList) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	if len(b) == 0 || bytes.Equal(b, []byte("null")) {
		*k = nil
		return nil
	}
	var raw []json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	out := make(KeyList, 0, len(raw))
	for _, item := range raw {
		item = bytes.TrimSpace(item)
		if len(item) > 0 && item[0] == '"' {
			var s string
			if err := json.Unmarshal(item, &s); err != nil {
				return err
			}
			out = append(out, s)
			continue
		}
		var obj struct {
			Key FlexString `json:"key"`
		}
		if err := json.Unmarshal(item, &obj); err != nil {
			return fmt.Errorf("expected key string or object, got %s", item)
		}
		out = append(out, string(obj.Key))
	}
	*k = out
	return nil
}

// FlexKey decodes a reference that is either a plain key string or an object
// with a "key" field. An empty FlexKey means the reference is unset.
type FlexKey string

func (f *FlexKey) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	if len(b) == 0 || bytes.Equal(b, []byte("null")) {
		*f = ""
		return nil
	}
	if b[0] == '"' {
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			return err
		}
		*f = FlexKey(s)
		return nil
	}
	var obj struct {
		Key FlexString `json:"key"`
	}
	if err := json.Unmarshal(b, &obj); err != nil {
		return fmt.Errorf("expected key string or object, got %s", b)
	}
	*f = FlexKey(obj.Key)
	return nil
}

// FlexText decodes a JSON string, or an array of strings joined by newlines.
type FlexText string

func (f *FlexText) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	if len(b) == 0 || bytes.Equal(b, []byte("null")) {
		*f = ""
		return nil
	}
	if b[0] == '[' {
		var items []string
		if err := json.Unmarshal(b, &items); err != nil {
			return err
		}
		*f = FlexText(strings.Join(items, "\n"))
		return nil
	}
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	*f = FlexText(s)
	return nil
}
