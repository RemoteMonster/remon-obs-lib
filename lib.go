package main

/*
#include <stdlib.h>
*/
import "C"
	
import (
	"unsafe"
	"fmt"
	"github.com/RemoteMonster/remon-obs-lib/Remon"
)

var gRm *Remon.Remon
var gVideoPtimeUs, gAudioPtimeUs float64
var gIsLastError = false
var gLastError *C.char
var gPeerID *C.char
var gChannelID *C.char


const (
	OBSERVER_MSG_INIT     = 0x0001
	OBSERVER_MSG_CREATE   = 0x0002
	OBSERVER_MSG_CONNECT  = 0x0004
	OBSERVER_MSG_COMPLETE = 0x0008
	OBSERVER_MSG_CLOSE    = 0x0010
	OBSERVER_MSG_ERROR    = 0x1000
	OBSERVER_MSG_ALL      = 0xFFFF
)

type ObserverMsg struct {
	Msg   uint32
	Value interface{}
}
type ObserverChan chan ObserverMsg

type LibObserver struct {
	observers map[ObserverChan]uint32
}

func NewLibObserver() *LibObserver {
	return &LibObserver{
		observers: make(map[ObserverChan]uint32, 3),
	}
}

func (lo *LibObserver) OnInit(token string) {
	fmt.Println("########## OnInit", token)
	lo.announce(OBSERVER_MSG_INIT, token)
}
func (lo *LibObserver) OnCreate(channelId string) {
	fmt.Println("########## OnCreate", channelId)
	lo.announce(OBSERVER_MSG_CREATE, channelId)
}
func (lo *LibObserver) OnComplete() {
	fmt.Println("########## OnComplete")
	lo.announce(OBSERVER_MSG_CREATE, nil)
}
func (lo *LibObserver) OnClose() {
	fmt.Println("########## OnClose")
	lo.announce(OBSERVER_MSG_CLOSE, nil)
}
func (lo *LibObserver) OnError(err error) {
	fmt.Println("########## OnError", err)
	remonSetLastError(err.Error())
	lo.announce(OBSERVER_MSG_ERROR, err.Error())
}
func (lo *LibObserver) announce(msg uint32, val interface{}) {
	for k, v := range lo.observers {
		if v&msg != 0 {
			k <- ObserverMsg{
				Msg:   msg,
				Value: val,
			}
		}
	}
}

//TODO: add timeout
func (lo *LibObserver) addObserver(ch ObserverChan, mask uint32) {
	lo.observers[ch] = mask
}
func (lo *LibObserver) removeObserver(ch ObserverChan) {
	delete(lo.observers, ch)
}

func remonSetLastError(msg string) {
	gIsLastError = true
	if gLastError != nil {
		C.free(unsafe.Pointer(gLastError))
	}
	gLastError = C.CString(msg)
}

func remonGetLastError() *C.char {
	if !gIsLastError {
		return nil
	}
	gIsLastError = false
	return gLastError
}

//export RemonCreateCast
func RemonCreateCast(serviceId, serviceKey, channelName string, videoPtimeUs, audioPtimeUs int64) (ChannelId *C.char, Pid *C.char, ErrorCode int) {
	gIsLastError = false
	obvr := NewLibObserver()
	gRm = Remon.New(Remon.Config{
		ServiceId: serviceId,
		Key:       serviceKey,
	}, obvr)
	gVideoPtimeUs = float64(videoPtimeUs)
	gAudioPtimeUs = float64(audioPtimeUs)

	ch := make(ObserverChan, 5)
	obvr.addObserver(ch, OBSERVER_MSG_ALL)
	defer obvr.removeObserver(ch)

	err := gRm.CreateCast(channelName)
	if err != nil {
		remonSetLastError(err.Error())
		return nil, nil, -1
	}
	//TODO: add timeout
	for {
		msg := <-ch
		switch msg.Msg {
		case OBSERVER_MSG_INIT:
			if gPeerID != nil {
				C.free(unsafe.Pointer(gPeerID))
			}
			gPeerID = C.CString(msg.Value.(string))
		case OBSERVER_MSG_CREATE:
			if gChannelID != nil {
				C.free(unsafe.Pointer(gChannelID))
			}
			gChannelID = C.CString(msg.Value.(string))
			return gChannelID, gPeerID, 0
		case OBSERVER_MSG_ERROR:
			return nil, nil, -1
		}
	}
}

//export RemonWriteVideo
func RemonWriteVideo(data []byte, ts uint64) (ErrorCode int) {
	if gIsLastError {
		return -1
	}
	ndata := make( []byte, len(data))
	copy( ndata, data)
	if( ts == 0) {
		gRm.WriteVideo(ndata, ts, 0)
	} else {
		gRm.WriteVideo(ndata, ts, gVideoPtimeUs)
	}
	return
}

//export RemonWriteAudio
func RemonWriteAudio(data []byte, ts uint64) (ErrorCode int) {
	if gIsLastError {
		return -1
	}
	ndata := make( []byte, len(data))
	copy( ndata, data)
	gRm.WriteAudio(ndata, ts, gAudioPtimeUs)
	return
}

//export RemonClose
func RemonClose() {
	gRm.Close()
	if gPeerID != nil {
		C.free(unsafe.Pointer(gPeerID))
		gPeerID = nil
	}
	if gChannelID != nil {
		C.free(unsafe.Pointer(gChannelID))
		gChannelID = nil
	}
	if gLastError != nil {
		C.free(unsafe.Pointer(gLastError))
		gLastError = nil
	}
}

//export RemonLastError
func RemonLastError() (ErrorMsg *C.char) {
	return remonGetLastError()
}


func main() {
}
