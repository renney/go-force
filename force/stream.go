package force

import (
	"encoding/json"
	"fmt"
	"golang.org/x/net/publicsuffix"
	"io/ioutil"
	"net/http"
	"net/http/cookiejar"
	//"reflect"
	"strings"
)

type forceStreaming struct {
	ClientId             string
	SubscribedPushTopics map[string]func([]byte, ...interface{})
	Timeout              int
	forceApi             *ForceApi
	longPollClient       *http.Client
}

func (s *forceStreaming) httpPost(payload string) (*http.Response, error) {
	ioPayload := strings.NewReader(payload)
	endpoint := s.forceApi.oauth.InstanceUrl + "/cometd/33.0" //version needs to be dynamic
	headerVal := "OAuth " + s.forceApi.oauth.AccessToken

	request, _ := http.NewRequest("POST", endpoint, ioPayload)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", headerVal)

	resp, err := s.longPollClient.Do(request)

	return resp, err
}

func (s *forceStreaming) connect() ([]byte, error) {
	connectParams := `{ "channel": "/meta/connect", "clientId": "` + s.ClientId + `", "connectionType": "long-polling"}`
	resp, err := s.httpPost(connectParams)
	respBytes, _ := ioutil.ReadAll(resp.Body)
	defer resp.Body.Close()

	return respBytes, err
}

func (forceApi *ForceApi) ConnectToStreamingApi() {
	//set up the client
	cookiejarOptions := cookiejar.Options{
		PublicSuffixList: publicsuffix.List,
	}
	jar, _ := cookiejar.New(&cookiejarOptions)
	forceApi.stream = &forceStreaming{"", map[string]func([]byte, ...interface{}){}, 0, forceApi, &http.Client{Jar: jar}}

	//handshake
	var params = `{"channel":"/meta/handshake", "supportedConnectionTypes":["long-polling"], "version":"1.0"}`
	handshakeResp, _ := forceApi.stream.httpPost(params)
	handshakeBytes, _ := ioutil.ReadAll(handshakeResp.Body)
	defer handshakeResp.Body.Close()

	var data []map[string]interface{}
	json.Unmarshal(handshakeBytes, &data)
	fmt.Println(data)
	forceApi.stream.ClientId = data[0]["clientId"].(string)

	//must handle error here

	// connect
	connBytes, _ := forceApi.stream.connect()

	var connectData []map[string]interface{}
	json.Unmarshal(connBytes, &connectData)
	for _, msg := range data {
		cb := forceApi.stream.SubscribedPushTopics[msg["channel"].(string)]
		if cb != nil {
			cb(connBytes)
		}
		fmt.Println(string(connBytes))
	}

	go func() {
		// got to allow disconnect, handle errors
		for {
			connBytes, _ = forceApi.stream.connect()
			json.Unmarshal(connBytes, &connectData)

			for _, msg := range connectData {
				cb := forceApi.stream.SubscribedPushTopics[msg["channel"].(string)]
				if cb != nil {
					cb(connBytes)
				}
			}
			//fmt.Println(string(connBytes))
		}
	}()
}

//here we have to allow the ability to pass in a callback function
func (forceApi *ForceApi) SubscribeToPushTopic(pushTopic string, callback func([]byte, ...interface{})) ([]byte, error) {
	topicString := "/topic/" + pushTopic
	subscribeParams := `{ "channel": "/meta/subscribe", "clientId": "` + forceApi.stream.ClientId + `", "subscription": "` + topicString + `"}`

	subscribeResp, _ := forceApi.stream.httpPost(subscribeParams)
	subscribeBytes, err := ioutil.ReadAll(subscribeResp.Body)

	defer subscribeResp.Body.Close()

	forceApi.stream.SubscribedPushTopics[topicString] = callback
	return subscribeBytes, err

}

func UnsubscribeFromPushTopic(pushTopic string) {
	fmt.Println(pushTopic)
}

func DisconnectStreamingApi() {
}
