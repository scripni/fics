package main

import (
        "bufio"
        "fmt"
        "log"
        "net"
        "strings"
        "unicode/utf8"
)

const addr = "freechess.org:5000"

func main() {
        log.Println("Begin")
        session, err := NewFicsSession()
        if err != nil {
                panic(err)
        }

        // TODO: defer close channels

        log.Println("Created session", session)
}

// holds a FICS telnet session
type FicsSession struct {

        // channel for reading incoming messages
        in      chan<- string

        // channel for writing outgoing messages
        out     <-chan string
}

// creates a new FICS session
func NewFicsSession() (*FicsSession, error) {

        log.Println("Connecting to", addr)

        // resolve host
        tcpAddr, resolveErr := net.ResolveTCPAddr("tcp", addr)
        if resolveErr != nil {
                return nil, fmt.Errorf("Error resolving address %v: %v", addr, resolveErr)
        }

        // connect
        conn, connErr := net.DialTCP("tcp", nil, tcpAddr)
        if connErr != nil {
                return nil, fmt.Errorf("Error connecting to %v: %v", tcpAddr, connErr)
        }

        // set a custom scanner splitting input by 'fics%'
        scanner := bufio.NewScanner(conn)
        const split = "fics%"
        scanner.Split(func(data []byte, atEOF bool) (advance int, token []byte, err error) {

                if i := strings.Index(string(data), split); i >= 0 {
                        // skip message plus token, read up to token, no error
                        return i + utf8.RuneCountInString(split), data[:i], nil
                }

                // TODO: atEOF == true
                return 0, nil, nil

        })

        // set up read channel
        in := make(chan string)
        go func() {
                for scanner.Scan() {
                        log.Println("Scanning for next message")
                        in <- scanner.Text()
                }

                // TODO: EOF, goroutine leak
                fmt.Println(scanner.Err())
        }()

        // set up write channel
        out := make(chan string)
        go func() {
                for {
                        // ignore number of bytes written
                        _, writeErr := conn.Write([]byte(<-out))
                        if writeErr != nil {
                                panic(writeErr)
                        }

                        // TODO: EOF, goroutine leak
                }
        }()

        return &FicsSession{
                in:     in,
                out:    out,
        }, nil
}
