package ssh

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strings"
)

// RespType is the status byte of an scp protocol response.
type RespType byte

const (
	Ok RespType = iota
	Warning
	Error
)

func parseResponse(r io.Reader) (RespType, string, error) {
	buffer := make([]byte, 1)
	_, err := r.Read(buffer)
	if err != nil {
		return Error, "", err
	}

	responseType := buffer[0]
	message := ""
	if responseType > 0 {
		bufferedReader := bufio.NewReader(r)
		message, err = bufferedReader.ReadString('\n')
		if err != nil {
			return Error, "", err
		}
	}

	return RespType(responseType), message, nil
}

func checkResponse(r io.Reader) error {
	respType, msg, err := parseResponse(r)
	if err != nil {
		return err
	}

	if respType != Ok {
		msg = strings.TrimSpace(msg)
		if msg == "" {
			return fmt.Errorf("remote scp returned status %d", respType)
		}
		return errors.New(msg)
	}

	return nil
}
