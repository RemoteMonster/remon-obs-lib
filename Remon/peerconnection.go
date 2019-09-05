package Remon

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"time"

	"github.com/pion/rtp"

	"github.com/pion/webrtc/v2/pkg/media"

	"github.com/pion/rtcp"
	"github.com/pion/webrtc/v2"
	"github.com/valyala/fastjson"
)

type peerconnection struct {
	config          rmConfig
	comm            chan commChan
	noti            chan commChan
	peerConnection  *webrtc.PeerConnection
	trackV          *webrtc.Track
	trackA          *webrtc.Track
	mediaAvailable  bool
	chanVideo       chan commMedia
	chanAudio       chan commMedia
	packetizerVideo rtp.Packetizer
	packetizerAudio rtp.Packetizer
}

//var videoPt uint8 = 100

func newPeerConnection(cfg rmConfig, notichan chan commChan) *peerconnection {
	var videoPt, audioPt uint8
	var videoCodec, audioCodec *webrtc.RTPCodec
	me := webrtc.MediaEngine{}

	switch cfg.audioCodec {
	case "g722":
		audioPt = webrtc.DefaultPayloadTypeG722
		audioCodec = webrtc.NewRTPG722Codec(audioPt, 8000)
	default:
		audioPt = webrtc.DefaultPayloadTypeOpus
		audioCodec = webrtc.NewRTPOpusCodec(audioPt, 48000)
	}
	switch cfg.videoCodec {
	case "vp8":
		videoPt = webrtc.DefaultPayloadTypeVP8
		videoCodec = webrtc.NewRTPVP8Codec(videoPt, 90000)
	case "vp9":
		videoPt = webrtc.DefaultPayloadTypeVP9
		videoCodec = webrtc.NewRTPVP9Codec(videoPt, 90000)
	default:
		videoPt = webrtc.DefaultPayloadTypeH264
		videoCodec = webrtc.NewRTPH264Codec(videoPt, 90000)
	}
	me.RegisterCodec(audioCodec)
	me.RegisterCodec(videoCodec)

	api := webrtc.NewAPI(webrtc.WithMediaEngine(me))

	pc := peerconnection{
		config:    cfg,
		comm:      make(chan commChan, 5),
		noti:      notichan,
		chanVideo: make(chan commMedia, 5),
		chanAudio: make(chan commMedia, 5),
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

	videoSSRC := rand.Uint32()
	audioSSRC := rand.Uint32()
	pc.packetizerVideo = rtp.NewPacketizer(1400, videoPt, videoSSRC, videoCodec.Payloader, rtp.NewRandomSequencer(), videoCodec.ClockRate)
	pc.packetizerAudio = rtp.NewPacketizer(1400, audioPt, audioSSRC, audioCodec.Payloader, rtp.NewRandomSequencer(), audioCodec.ClockRate)

	pc.trackV, err = peerConnection.NewTrack(videoPt, videoSSRC, "video", "pion")
	if err != nil {
		panic(err)
	}
	// Add this newly created track to the PeerConnection
	if _, err = peerConnection.AddTrack(pc.trackV); err != nil {
		panic(err)
	}
	pc.trackA, err = peerConnection.NewTrack(audioPt, audioSSRC, "audio", "pion")
	if err != nil {
		panic(err)
	}
	// Add this newly created track to the PeerConnection
	if _, err = peerConnection.AddTrack(pc.trackA); err != nil {
		panic(err)
	}

	peerConnection.OnTrack(func(track *webrtc.Track, receiver *webrtc.RTPReceiver) {
		fmt.Printf("################### OnTrack ID:%s\n", track.ID())
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
			rtpsenders := peerConnection.GetSenders()
			for i := 0; i < len(rtpsenders); i++ {
				go rtcpReaderLoop(&pc, i, rtpsenders[i])
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
	go writeVideoLoop(&pc)
	go writeAudioLoop(&pc)
	return &pc
}

func rtcpReaderLoop(pc *peerconnection, idx int, rtpsender *webrtc.RTPSender) {
	log.Println("rtcpReaderLoop+")
	defer log.Println("rtcpReaderLoop-")
	for {
		packets, err := rtpsender.ReadRTCP()
		if err != nil {
			log.Printf("read [%d] err: %s\n", idx, err.Error())
			return
		} else {
			for _, packet := range packets {
				switch rtcp_type := packet.(type) {
				case *rtcp.ReceiverEstimatedMaximumBitrate:
					{
						remb := packet.(*rtcp.ReceiverEstimatedMaximumBitrate)
						_ = remb
						// log.Printf("read [%d] ReceiverEstimatedMaximumBitrate %d\n", idx, remb.Bitrate)
					}
				case *rtcp.PictureLossIndication:
					{
						pli := packet.(*rtcp.PictureLossIndication)
						_ = pli
						// log.Printf("read [%d] PictureLossIndication %d\n", idx, pli.MediaSSRC)
					}
				case *rtcp.ReceiverReport:
					{
						rr := packet.(*rtcp.ReceiverReport)
						for _, report := range rr.Reports {
							_ = report
							// log.Printf("read [%d] ReceiverReport fractionLost:%f totalLost:%d\n", idx, float64(report.FractionLost)/256.0, report.TotalLost)
						}
					}
				case *rtcp.SenderReport:
					{
						_ = packet.(*rtcp.SenderReport)
						// log.Printf("read [%d] SenderReport\n", idx)
					}
				default:
					{
						_ = rtcp_type
						//	log.Printf("read [%d] unknown %v\n", idx, rtcp_type)
					}
				}
			}
		}
	}
}

func peerConnectionLoop(pc *peerconnection) {
	log.Println("peerConnectionLoop+")
	defer func() {
		close(pc.noti)
		close(pc.chanVideo)
		close(pc.chanAudio)
		log.Println("peerConnectionLoop-")
	}()
	for {
		select {
		case msg, ok := <-pc.comm:
			//log.Printf("peerConnectionLoop cmd:%s\n", msg.cmd)
			if !ok {
				pc.peerConnection.Close()
				return
			}
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

func writeVideoLoop(pc *peerconnection) {
	log.Println("writeVideoLoop+")
	defer log.Println("writeVideoLoop-")
	for {
		select {
		case media, ok := <-pc.chanVideo:
			{
				if !ok {
					return
				}
				samples := uint32(videoClockRate * (media.duration / 1000000000))
				packets := pc.packetizerVideo.Packetize(*media.data, samples)
				for _, p := range packets {
					err := pc.trackV.WriteRTP(p)
					if err != nil {
						log.Printf("Write RTP error : %v\n", err)
					}
					time.Sleep(time.Microsecond * 1)
				}
				/*
					pc.comm <- commChan{
						cmd: "media",
						media: &commMedia{
							audio:    false,
							data:     media.data,
							ts:       media.ts,
							duration: media.duration,
						},
					}
				*/
			}
		}
	}
}
func writeAudioLoop(pc *peerconnection) {
	log.Println("writeAudioLoop+")
	defer log.Println("writeAudioLoop-")
	for {
		select {
		case media, ok := <-pc.chanAudio:
			{
				if !ok {
					return
				}
				pc.comm <- commChan{
					cmd: "media",
					media: &commMedia{
						audio:    true,
						data:     media.data,
						ts:       media.ts,
						duration: media.duration,
					},
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
