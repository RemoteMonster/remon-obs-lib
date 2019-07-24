package Remon

type msgInitRequest struct {
	Credential msgCredential `json:"credential"`
	Env        msgEnv        `json:"env"`
}

type msgInitResponse struct {
	SigURL     string         `json:"sigurl"`
	IceServers []msgIceServer `json:"iceServers"`
	Token      string         `json:"token"`
	Key        string         `json:"key"`
	Name       string         `json:"name"`
}

type msgCredential struct {
	Key       string `json:"key"`
	ServiceId string `json:"serviceId"`
}

type msgEnv struct {
	Os            string `json:"os"`
	OsVersion     string `json:"osVersion"`
	Device        string `json:"device"`
	DeviceVersion string `json:"deviceVersion"`
	NetworkType   string `json:"networkType"`
	SdkVersion    string `json:"sdkVersion"`
}

type msgIceServer struct {
	Urls       string `json:"urls"`
	Credential string `json:"credential"`
	Username   string `json:"username"`
}

type msgEventRequest struct {
	Topic    string          `json:"topic"`
	Messages msgEventMessage `json:"messages"`
}

type msgEventMessage struct {
	Log      string `json:"log"`
	LogLevel string `json:"logLevel"`
	SvcId    string `json:"svcId"`
	PId      string `json:"pId"`
	Status   string `json:"status"`
	msgEnv
}

//===========================================================

type msgSignalRequest struct {
	Command   string         `json:"command,omitempty"`
	Token     string         `json:"token,omitempty"`
	ServiceId string         `json:"serviceId,omitempty"`
	Channel   *signalChannel `json:"channel,omitempty"`
	Body      *string        `json:"body,omitempty"`
}

type msgSignalResponse struct {
	msgSignalRequest
	Code     string `json:"code,omitempty"`
	Receiver string `json:"receiver,omitempty"`
}

type signalChannel struct {
	MotherId string `json:"motherId,omitempty"`
	Id       string `json:"id,omitempty"`
	Name     string `json:"name,omitempty"`
	Type     string `json:"type,omitempty"`
}

// 모든 값이 default일때 msgSignalRequest json encoding시 "channel" key가 아예 나오지 않게 하기 위한 편의 함수
// 나와도 될 것 같긴 함..
func (sc *signalChannel) GetPointer() *signalChannel {
	if sc.MotherId == "" && sc.Id == "" && sc.Name == "" && sc.Type == "" {
		return nil
	}
	return sc
}
