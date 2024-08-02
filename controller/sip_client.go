package main

import (
	"net"
	"bufio"
	"errors"
	"strings"
	"strconv"
	"fmt"
	"github.com/google/uuid"
)

func multiline(parts ...string) string {
    return strings.Join(parts, "\r\n")
}

func SipSendMessage(host string, body string) (int, error) {
	uuid := uuid.NewString()
	sock, err := net.Dial("udp", host)
	defer sock.Close()
	sourceAddr := sock.LocalAddr()
	fmt.Printf("\n%s\n", sourceAddr) 
	// sourcePort := strings.Split(sourceAddr.String(), ":")[1]
	if err != nil {
		fmt.Printf("error: %s", err)
		return 503, err
	}

	message :=  multiline (
		"MESSAGE sip:echo@15.222.241.45:5062 SIP/2.0",
		fmt.Sprintf("Via: SIP/2.0/UDP %s;branch=z9hG4bK-94788c76811a", sourceAddr),
		fmt.Sprintf("To: <sip:%s>", host),
		"From: <sip:hct@controller.void:5060>;tag=5b60ada2e3c4",
		"CSeq: 1 MESSAGE",
		fmt.Sprintf("Call-ID: %s", uuid),
		"Max-Forwards: 70",
		"User-Agent: hct_controller",
		fmt.Sprintf("Content-Length: %d", len(body)),
		"Content-Type: text/json",
		"",
		fmt.Sprintf("%s", body),
	)
	fmt.Fprintf(sock, message)
	fmt.Printf("sending[%s]: \n\n%s\n\n", host, message)
	p :=  make([]byte, 2048)
	_, err = bufio.NewReader(sock).Read(p)
	if err == nil {
		if len(p) < 12 || string(p[0:3]) != "SIP" {
			return 503, errors.New("invalid SIP response.\n")
		}
		code, e := strconv.Atoi(string(p[8:11]))
		if e != nil {
			return 503, errors.New("invalid SIP code.\n")
		}
		// fmt.Printf("%s\n", p)
		return code, nil
	} else {
		fmt.Printf("error reading socket: %v\n", err)
		return 503, err
	}
	return 503, nil
}

func AllowIp(host string, ip string) (int, error) {
	cmd := fmt.Sprintf("{ \"cmd\": \"allow\", \"ip\": \"%s\" }", ip);
	return SipSendMessage(host, cmd)
}
