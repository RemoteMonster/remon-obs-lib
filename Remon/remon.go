package Remon

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
)

type Remon struct {
	//	state  int
	config  rmConfig
	signal  *signalConnection
	started bool
}

type Config struct {
	ServiceId string
	Key       string
}

// Observer : Remon Observer interface
type Observer interface {
	OnInit(token string)
	OnCreate(channelId string)
	//OnJoin()
	//OnConnect(channelId string)
	OnComplete()
	OnClose()
	OnError(err error)
	//OnMessage(msg1 string, msg2 string)
	// OnStat(...)
}

func New(config Config, observer Observer) *Remon {
	rm := Remon{}
	rm.config = defaultConfig()

	rm.config.credconfig.serviceId = config.ServiceId
	rm.config.credconfig.key = config.Key
	rm.config.observer = observer
	return &rm
}

// CreateCast : create cast
func (rm *Remon) CreateCast(name string) error {
	rm.started = true
	err := rm.init()
	if err != nil {
		return err
	}
	err = rm.signal.createBroadcastChannel(name)
	return err
}

// JoinCast :
func (rm *Remon) JoinCast(roomID string) error {
	rm.config.mediaconfig.recvOnly = true
	rm.config.channel.Type = "VIEWER"

	err := rm.init()
	if err != nil {
		return err
	}
	err = rm.signal.createViewerChannel(roomID)
	return err
}

func (rm *Remon) Close() {
	rm.started = false
	rm.signal.close()
}

func (rm *Remon) WriteVideo(data []byte, timestamp uint64, duration float64) {
	if rm.started {
		rm.signal.pc.chanVideo <- commMedia{
			audio:    false,
			data:     &data,
			ts:       timestamp,
			duration: duration,
		}
	}
}

func (rm *Remon) WriteAudio(data []byte, timestamp uint64, duration float64) {
	if rm.started {
		rm.signal.pc.chanAudio <- commMedia{
			audio:    true,
			data:     &data,
			ts:       timestamp,
			duration: duration,
		}
	}
}

// FetchCasts retrieve all broadcast rooms information
func (rm *Remon) FetchCasts() ([]Room, error) {
	url := rm.config.appServer + "/room/" + rm.config.credconfig.serviceId
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	respByte, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Println("fail to read response data")
		return nil, err
	}
	var rooms []Room
	err = json.Unmarshal(respByte, &rooms)
	if err != nil {
		return nil, err
	}
	return rooms, nil
}

func (rm *Remon) init() error {
	//log.Println("init")
	msg := msgInitRequest{
		Credential: msgCredential{
			Key:       rm.config.credconfig.key,
			ServiceId: rm.config.credconfig.serviceId,
		},
		Env: msgEnv{
			SdkVersion: rm.config.sdkconfig.version,
		},
	}
	jsonBytes, err := json.Marshal(&msg)
	if err != nil {
		//log.Printf("[FATAL]")
		return err
	}
	//log.Println("init: send: " + string(jsonBytes))

	url := rm.config.appServer + "/init"
	resp, err := http.Post(url, "application/json", bytes.NewReader(jsonBytes))
	if err != nil {
		//log.Printf("[FATAL]")
		return err
	}
	defer resp.Body.Close()

	respByte, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Println("fail to read response data")
		return err
	}
	//log.Println("init: recv: " + string(respByte))

	var initResp msgInitResponse
	err = json.Unmarshal(respByte, &initResp)
	if err != nil {
		//log.Printf("[FATAL]")
		return err
	}
	if len(initResp.IceServers) > 0 {
		rm.config.rtcconfig.iceServers = initResp.IceServers
	}
	if initResp.Token != "" {
		rm.config.token = initResp.Token
	}
	if initResp.SigURL != "" {
		rm.config.signalingServer = initResp.SigURL
	}
	if initResp.Name != "" {
		rm.config.channel.Name = initResp.Name
	}
	if initResp.Key != "" {
		rm.config.channel.Id = initResp.Key
	}

	if rm.config.observer != nil {
		rm.config.observer.OnInit(initResp.Token)
	}

	rm.signal = newSignalConnection(rm.config)
	err = rm.signal.connect()

	return err
}
