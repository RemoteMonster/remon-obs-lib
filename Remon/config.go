package Remon

type rmConfig struct {
	rtcconfig       rtcConfig
	signalingServer string
	appServer       string
	logServer       string
	sdkconfig       sdkConfig
	credconfig      credConfig
	viewconfig      viewConfig
	mediaconfig     mediaConfig

	token    string
	channel  signalChannel
	observer Observer
}

type rtcConfig struct {
	iceServers   []msgIceServer
	simulcast    bool
	sdpSemantics string
}

type sdkConfig struct {
	loglevel string
	contry   string
	version  string
}

type credConfig struct {
	key       string
	serviceId string
}

type viewConfig struct {
	local  bool
	remote bool
}

type mediaConfig struct {
	video    bool
	audio    bool
	record   bool
	recvOnly bool
}

func defaultConfig() rmConfig {
	cfg := rmConfig{
		rtcconfig: rtcConfig{
			iceServers: []msgIceServer{
				msgIceServer{
					Urls: "stun:stun.l.google.com:19302",
				},
			},
			simulcast:    false,
			sdpSemantics: "unified-plan",
		},
		sdkconfig: sdkConfig{
			version: "2.3.10",
		},
		signalingServer: "wss://signal.remotemonster.com/ws",
		appServer:       "https://signal.remotemonster.com/rest",
		logServer:       "https://signal.remotemonster.com:2001/topics",
		mediaconfig: mediaConfig{
			video:  true,
			audio:  true,
			record: false,
		},
	}
	return cfg
}
