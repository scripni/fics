package main

import (
        "bufio"
        "fmt"
        "github.com/gorilla/websocket"
        "log"
        "net"
        "net/http"
)

const addr = "freechess.org:5000"

var upgrader = websocket.Upgrader{} // use default options

// TODO: parse config values from flags
func main() {
        log.Println("Starting server")
        http.HandleFunc("/ws", wsHandler)
        fs := http.FileServer(http.Dir("/bootstrap"))
        http.Handle("/bootstrap", http.StripPrefix("/bootstrap", fs))
        http.Handle("/", http.FileServer(http.Dir("./html")))

        log.Fatal(http.ListenAndServe("localhost:3030", nil))
}

// holds a FICS telnet session
type FicsSession struct {
        // channel for reading incoming messages
        read    <-chan []byte
        // channel for writing outgoing messages
        write   chan<- []byte
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
        var split = []byte("fics%")
        scanner.Split(func(data []byte, atEOF bool) (advance int, token []byte, err error) {

                if i := IndexBytes(data, split); i >= 0 {
                        out := make([]byte, 0, i)
                        for j := 0; j < i; j++ {
                                if data[j] <= 0x7f {
                                        out = append(out, data[j])
                                }
                        }

                        // skip message plus token, read up to token, no error
                        return i + len(split), out, nil
                }

                // TODO: atEOF == true
                return 0, nil, nil

        })

        // set up read channel
        in := make(chan []byte)
        go func() {
                for scanner.Scan() {
                        in <- scanner.Bytes()
                }

                // TODO: EOF, goroutine leak
                fmt.Println(scanner.Err())
        }()

        // set up write channel
        out := make(chan []byte)
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
                read:   in,
                write:  out,
        }, nil
}

// finds the first occurence of a subarray
func IndexBytes(message []byte, token []byte) int {
        i, j := 0, 0
        for ; i < len(message) && j < len(token); i++ {
                if message[i] == token[j] {
                        j++
                } else {
                        j = 0
                }
        }

        if j == len(token) {
                return i - len(token)
        } else {
                return -1
        }
}

// connects the user as a guest
func (session *FicsSession) ConnectGuest() {

        log.Println("Signing in as guest")

        // HACK: send 10 '\n' chars otherwise no answer comes back
        // might be a buffer that needs flushed, TCPConn.SetWriteBuffer(1)
        // doesn't seem to work.
        // TODO: ask Mario
        session.write <- []byte("g\n\n\n\n\n\n\n\n\n\n")
}

// handles an incoming websocket connection
func wsHandler(w http.ResponseWriter, r *http.Request) {
        c, err := upgrader.Upgrade(w, r, nil)
        if err != nil {
                log.Print("upgrade:", err)
                return
        }
        defer c.Close()

        s, err := NewFicsSession()
        if err != nil {
                panic(err)
        }
        // TODO: defer close channels

        log.Println("Created session", s)
        s.ConnectGuest()
        log.Println("Connected as guest")

        // write out all messages
        for msg := range s.read {
                err = c.WriteMessage(websocket.TextMessage, msg)
                if err != nil {
                        log.Println("error writing message:", err)
                        break
                }
        }
}
