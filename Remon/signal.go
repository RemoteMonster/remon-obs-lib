package Remon

import (
	"encoding/json"

	//"golang.org/x/net/websocket"
	"github.com/gorilla/websocket"
)

type signalConnection struct {
	comm   chan commChan
	config rmConfig
	conn   *websocket.Conn
	pc     *peerconnection
	closed bool
}

func newSignalConnection(config rmConfig) *signalConnection {
	return &signalConnection{
		comm:   make(chan commChan, 5),
		config: config,
	}
}

func (sc *signalConnection) connect() error {
	var err error
	sc.conn, _, err = websocket.DefaultDialer.Dial(sc.config.signalingServer, nil)
	//log.Printf("signal connect url=%s err=%v", sc.config.signalingServer, err)
	if err != nil {
		return err
	}
	sc.pc = newPeerConnection(sc.config, sc.comm)
	go wsReadLoop(sc)
	go pcReadLoop(sc)
	return nil
}

func (sc *signalConnection) createBroadcastChannel(name string) error {
	//log.Printf("createBroadcastChannel %s", name)
	sc.config.channel.Type = "BROADCAST"
	sc.config.channel.Name = name
	msg, err := sc.makeMessage("create", nil)
	if err != nil {
		return err
	}
	err = sc.conn.WriteMessage(websocket.TextMessage, msg)
	if err != nil {
		panic(err)
	}

	return err
}

func (sc *signalConnection) createViewerChannel(id string) error {
	sc.config.mediaconfig.recvOnly = true
	sc.config.channel.Type = "VIEWER"
	sc.config.channel.Id = id
	msg, err := sc.makeMessage("create", nil)
	if err != nil {
		return err
	}
	//log.Printf("SEND: %s", string(msg))
	err = sc.conn.WriteMessage(websocket.TextMessage, msg)
	if err != nil {
		return err
	}
	return nil
}

func (sc *signalConnection) close() error {
	sc.pc.comm <- commChan{
		cmd: "mediastop",
	}
	msg, err := sc.makeMessage("disconnect", nil)
	if err != nil {
		return err
	}
	//log.Printf("SEND: %s", string(msg))
	err = sc.conn.WriteMessage(websocket.TextMessage, msg)
	//TODO: all should be async
	sc.quit()
	if err != nil {
		return err
	}
	return nil
}

func (sc *signalConnection) quit() {
	// make pcReadLoop stop
	sc.comm <- commChan{
		cmd: "quit",
	}
	sc.closed = true
	if sc.config.observer != nil {
		sc.config.observer.OnClose()
	}
}

func (sc *signalConnection) makeMessage(cmd string, body *string) ([]byte, error) {
	msg := msgSignalRequest{
		Command:   cmd,
		Token:     sc.config.token,
		ServiceId: sc.config.credconfig.serviceId,
		Channel:   sc.config.channel.GetPointer(),
		Body:      body,
	}
	jsonByte, err := json.Marshal(msg)
	if err != nil {
		return nil, err
	}
	return jsonByte, nil
}

// Read websocket message
func wsReadLoop(sc *signalConnection) {
	ws := sc.conn

	ws.SetReadLimit(100000)
	//	ws.SetReadDeadline(time.Now().Add(pongWait))
	//	ws.SetPongHandler(func(string) error {
	//		ws.SetReadDeadline(time.Now().Add(pongWait))
	//		return nil
	//	})
	for {
		var wsResp msgSignalResponse
		if sc.closed {
			break
		}
		_, buf, err := ws.ReadMessage()
		if err != nil {
			break
		}
		err = json.Unmarshal(buf, &wsResp)
		if err != nil {
			panic(err)
		}
		switch wsResp.Command {
		case "ping":
			{
				wsResp.Command = "pong"
				jsonByte, err := json.Marshal(wsResp)
				if err != nil {
					panic(err)
				}
				err = ws.WriteMessage(websocket.TextMessage, jsonByte)
			}
		case "onCreate":
			{
				//log.Printf("wsResp onCreate\n")
				// join 접속시 chid
				if wsResp.Channel != nil {
					sc.config.channel.MotherId = wsResp.Channel.MotherId
					sc.config.channel.Id = wsResp.Channel.Id
					sc.config.channel.Name = wsResp.Channel.Name
				}
				if sc.config.observer != nil {
					sc.config.observer.OnCreate(sc.config.channel.Id)
				}
				// create offer?
				if sc.config.channel.Type == "VIEWER" {
					sc.pc.comm <- commChan{
						cmd: "fakeoffer",
					}
				} else {
					sc.pc.comm <- commChan{
						cmd: "createoffer",
					}
				}

			}
		case "onSdp":
			{
				//log.Printf("wsResp onSdp\n")
				sc.pc.comm <- commChan{
					cmd:  "onSdp",
					body: *wsResp.Body,
				}
			}
		case "onIce":
			{
				//log.Printf("onIce not implemented\n")
				sc.pc.comm <- commChan{
					cmd:  "onIce",
					body: *wsResp.Body,
				}
			}
		}

	}
	ws.Close()
}

func pcReadLoop(sc *signalConnection) {
	for {
		select {
		case msg := <-sc.comm:
			//log.Printf("pcReadLoop cmd:%s\n", msg.cmd)
			switch msg.cmd {
			case "sdp":
				{
					msg, err := sc.makeMessage("sdp", &msg.body)
					if err != nil {
						panic(err)
					}
					err = sc.conn.WriteMessage(websocket.TextMessage, msg)
					if err != nil {
						panic(err)
					}
				}
			case "quit":
				{
					return
				}
			}

		}
	}
}
