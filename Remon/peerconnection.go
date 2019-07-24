package Remon

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"time"

	"github.com/pion/webrtc/v2/pkg/media"

	"github.com/pion/rtcp"
	"github.com/pion/webrtc/v2"
	"github.com/valyala/fastjson"
)

type peerconnection struct {
	config         rmConfig
	comm           chan commChan
	noti           chan commChan
	peerConnection *webrtc.PeerConnection
	trackV         *webrtc.Track
	trackA         *webrtc.Track
	mediaAvailable bool
}

//var videoPt uint8 = webrtc.DefaultPayloadTypeH264
var videoPt uint8 = 100

func newPeerConnection(cfg rmConfig, notichan chan commChan) *peerconnection {

	me := webrtc.MediaEngine{}
	me.RegisterCodec(webrtc.NewRTPOpusCodec(webrtc.DefaultPayloadTypeOpus, 48000))
	//me.RegisterCodec(webrtc.NewRTPVP8Codec(webrtc.DefaultPayloadTypeVP8, 90000))
	me.RegisterCodec(webrtc.NewRTPH264Codec(videoPt, 90000))
	api := webrtc.NewAPI(webrtc.WithMediaEngine(me))

	pc := peerconnection{
		config: cfg,
		comm:   make(chan commChan, 5),
		noti:   notichan,
	}

	var iceservers []webrtc.ICEServer
	for _, v := range cfg.rtcconfig.iceServers {
		item := webrtc.ICEServer{
			URLs:           []string{v.Urls},
			Username:       v.Username,
			Credential:     v.Credential,
			CredentialType: webrtc.ICECredentialTypePassword,
		}
		iceservers = append(iceservers, item)
	}
	config := webrtc.Configuration{
		ICEServers:   iceservers,
		SDPSemantics: webrtc.SDPSemanticsPlanB,
	}
	peerConnection, err := api.NewPeerConnection(config)
	if err != nil {
		panic(err)
	}
	/*
		//videoTranceiver, err := peerConnection.AddTransceiver(webrtc.RTPCodecTypeVideo, webrtc.RtpTransceiverInit{Direction: webrtc.RTPTransceiverDirectionRecvonly})
		//audioTranceiver, err := peerConnection.AddTransceiver(webrtc.RTPCodecTypeAudio, webrtc.RtpTransceiverInit{Direction: webrtc.RTPTransceiverDirectionRecvonly})
		videoTranceiver, err := peerConnection.AddTransceiver(webrtc.RTPCodecTypeVideo, webrtc.RtpTransceiverInit{Direction: webrtc.RTPTransceiverDirectionSendrecv})
		audioTranceiver, err := peerConnection.AddTransceiver(webrtc.RTPCodecTypeAudio, webrtc.RtpTransceiverInit{Direction: webrtc.RTPTransceiverDirectionSendrecv})

		_ = videoTranceiver
		_ = audioTranceiver
	*/
	// Create Track that we send video back to browser on

	pc.trackV, err = peerConnection.NewTrack(videoPt, rand.Uint32(), "video", "pion")
	if err != nil {
		panic(err)
	}
	// Add this newly created track to the PeerConnection
	if _, err = peerConnection.AddTrack(pc.trackV); err != nil {
		panic(err)
	}

	pc.trackA, err = peerConnection.NewTrack(webrtc.DefaultPayloadTypeOpus, rand.Uint32(), "audio", "pion")
	if err != nil {
		panic(err)
	}
	// Add this newly created track to the PeerConnection
	if _, err = peerConnection.AddTrack(pc.trackA); err != nil {
		panic(err)
	}

	peerConnection.OnTrack(func(track *webrtc.Track, receiver *webrtc.RTPReceiver) {
		// Send a PLI on an interval so that the publisher is pushing a keyframe every rtcpPLIInterval
		// This is a temporary fix until we implement incoming RTCP events, then we would push a PLI only when a viewer requests it
		go func() {
			ticker := time.NewTicker(time.Second * 3)
			for range ticker.C {
				errSend := peerConnection.WriteRTCP([]rtcp.Packet{&rtcp.PictureLossIndication{MediaSSRC: track.SSRC()}})
				if errSend != nil {
					//fmt.Println(errSend)
				}
			}
		}()
		for {
			// Read RTP packets being sent to Pion
			_, readErr := track.ReadRTP()
			if readErr != nil {
				panic(readErr)
			}
			//fmt.Printf("READ RTP %d\n", rtp.SequenceNumber)
		}
	})
	// Set the handler for ICE connection state
	// This will notify you when the peer has connected/disconnected
	peerConnection.OnICEConnectionStateChange(func(connectionState webrtc.ICEConnectionState) {
		log.Printf("Connection State has changed %s \n", connectionState.String())
		if connectionState == webrtc.ICEConnectionStateConnected {
			pc.noti <- commChan{
				cmd:  "stateChange",
				body: "COMPLETE",
			}
			if pc.config.observer != nil {
				pc.config.observer.OnComplete()
			}
			if pc.config.channel.Type != "VIEWER" {
				pc.comm <- commChan{
					cmd: "startmedia",
				}
			}
		}
	})

	peerConnection.OnICEGatheringStateChange(func(gs webrtc.ICEGathererState) {
		log.Printf("OnICEGatheringStateChange %s \n", gs.String())
	})

	peerConnection.OnICECandidate(func(ic *webrtc.ICECandidate) {
		if ic == nil {
			//log.Printf("ICE Candidate END \n")
		} else {
			//log.Printf("ICE Candidate: %s \n", ic.String())
		}
	})

	pc.peerConnection = peerConnection
	go peerConnectionLoop(&pc)
	return &pc
}

func peerConnectionLoop(pc *peerconnection) {
	for {
		select {
		case msg := <-pc.comm:
			//log.Printf("peerConnectionLoop cmd:%s\n", msg.cmd)
			switch msg.cmd {
			case "onSdp":
				{
					var p fastjson.Parser
					v, err := p.Parse(msg.body)
					if err == nil {
						sdpType := v.GetStringBytes("type")
						sdp := v.GetStringBytes("sdp")
						//log.Printf("onSdp = %s\n", msg.body)
						pionDesc := webrtc.SessionDescription{
							SDP: string(sdp),
						}
						if string(sdpType) == "offer" {
							pionDesc.Type = webrtc.SDPTypeOffer
						} else {
							pionDesc.Type = webrtc.SDPTypeAnswer
						}
						//log.Printf("remote sdpType=%s", sdpType)
						//log.Printf("remote sdp=\n========\n%s\n=========", sdp)
						err = pc.peerConnection.SetRemoteDescription(pionDesc)
						if err != nil {
							panic(err)
						}
						//time.Sleep(3 * time.Second)
						if string(sdpType) == "offer" {
							pionDesc, err = pc.peerConnection.CreateAnswer(nil)
							if err != nil {
								panic(err)
							}
							pc.peerConnection.SetLocalDescription(pionDesc)
							jsonByte, err := json.Marshal(pionDesc)
							if err != nil {
								panic(err)
							}
							pc.noti <- commChan{
								cmd:  "sdp",
								body: string(jsonByte),
							}
							//log.Printf("local answer SDP=\n========\n%s\n=========", pionDesc.SDP)
						}
					} else {
						panic(err)
					}
				}
			case "createoffer":
				{
					pionDesc, err := pc.peerConnection.CreateOffer(nil)
					if err != nil {
						panic(err)
					}
					err = pc.peerConnection.SetLocalDescription(pionDesc)
					if err != nil {
						panic(err)
					}

					jsonByte, err := json.Marshal(pionDesc)
					if err != nil {
						panic(err)
					}
					pc.noti <- commChan{
						cmd:  "sdp",
						body: string(jsonByte),
					}
				}
			case "fakeoffer":
				{
					var fc webrtc.SessionDescription
					fc.Type = webrtc.SDPTypeOffer
					fc.SDP = "v=0\r\no=- 801243091147111611 2 IN IP4 127.0.0.1\r\ns=-\r\nt=0 0\r\na=group:BUNDLE 0 1\r\na=msid-semantic: WMS\r\n"

					jsonByte, err := json.Marshal(fc)
					if err != nil {
						panic(err)
					}
					pc.noti <- commChan{
						cmd:  "sdp",
						body: string(jsonByte),
					}
				}
			case "startmedia":
				{
					pc.mediaAvailable = true
				}
			case "stopmedia":
				{
					pc.mediaAvailable = false
				}
			case "media":
				{
					if pc.mediaAvailable {
						sample := media.Sample{
							Data: *msg.media.data,
						}
						if msg.media.audio {
							sample.Samples = uint32(audioClockRate * (msg.media.duration / 1000000000))
							pc.trackA.WriteSample(sample)
						} else {
							if msg.media.ts == 0 {
								sample.Samples = 0
							} else {
								sample.Samples = uint32(videoClockRate * (msg.media.duration / 1000000000))
							}
							pc.trackV.WriteSample(sample)
							//dumpBytes(*msg.media.data, 12)
						}
					}
				}
			}
		}
	}
}

func dumpBytes(data []byte, max int) {
	var buf bytes.Buffer
	buf.WriteString(fmt.Sprintf("write %d bytes - ", len(data)))
	if max > len(data) {
		max = len(data)
	}
	for i := 0; i < max; i++ {
		buf.WriteString(fmt.Sprintf("%02x ", data[i]))
	}
	buf.WriteString("\n")
	fmt.Printf(string(buf.String()))
}

var (
	videoClockRate float64 = 90000
	audioClockRate float64 = 48000
)
