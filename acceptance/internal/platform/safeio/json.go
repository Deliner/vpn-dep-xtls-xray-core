package safeio

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
)

func Encode(value any) ([]byte, error) {
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	err := encoder.Encode(value)
	return bytes.TrimSpace(buffer.Bytes()), err
}

func Canonical(value any) ([]byte, error) {
	raw, err := Encode(value)
	if err != nil {
		return nil, err
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var generic any
	if err := decoder.Decode(&generic); err != nil {
		return nil, err
	}
	return Encode(generic)
}

func Digest(value any) (string, error) {
	raw, err := Canonical(value)
	return Hash(raw), err
}

func JSON(path string, value any) error {
	raw, err := formattedJSON(value)
	if err != nil {
		return err
	}
	return Atomic(path, raw, 0600)
}

func NewJSON(path string, value any) error {
	raw, err := formattedJSON(value)
	if err != nil {
		return err
	}
	return Create(path, raw, 0600)
}

func formattedJSON(value any) ([]byte, error) {
	raw, err := Encode(value)
	if err != nil {
		return nil, err
	}
	var formatted bytes.Buffer
	if err := json.Indent(&formatted, raw, "", "  "); err != nil {
		return nil, err
	}
	return append(formatted.Bytes(), '\n'), nil
}

func LoadJSON(path string, value any) error {
	raw, err := Read(path, 10_000_000)
	if err != nil {
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	decoder.UseNumber()
	if err := decoder.Decode(value); err != nil {
		return fmt.Errorf("invalid JSON structure in %s", path)
	}
	if decoder.Decode(new(any)) != io.EOF {
		return fmt.Errorf("unexpected trailing JSON in %s", path)
	}
	return nil
}
