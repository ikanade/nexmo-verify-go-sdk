package nexmoVerifySDK

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"time"
)

var endpointUrls = map[string]string{
	"getToken":      "https://api.nexmo.com/sdk/token/json",
	"verifySearch":  "https://api.nexmo.com/sdk/verify/search/json",
	"verifyRequest": "https://api.nexmo.com/verify/json",
	"verifyCheck":   "https://api.nexmo.com/verify/check/json",
}

var tokenReplaceRegexp, _ = regexp.Compile("[&,=]")

func NewClient(appId, sharedSecret string) *Client {
	// Faking requests to act as the ANDROID SDK
	return &Client{
		appId:        appId,
		osFamily:     "ANDROID",
		sdkRevision:  "1.0",
		osRevision:   "23",
		sharedSecret: sharedSecret,
	}
}

func NewClientV2(apiKey, apiSecret string) *ClientV2 {
	// Creates client with apikey and apisecret used for newest nexmo-api
	return &ClientV2{
		apiKey:    apiKey,
		apiSecret: apiSecret,
	}
}

type Client struct {
	appId        string
	osFamily     string
	osRevision   string
	sdkRevision  string
	sharedSecret string
}

type ClientV2 struct {
	apiKey    string
	apiSecret string
}

type BaseResponse struct {
	ResultCode    int    `json:"result_code"`
	ResultMessage string `json:"result_message"`
	Timestamp     string `json:"timestamp"`
}

type VerifySearchResponse struct {
	BaseResponse
	UserStatus string `json:"user_status"`
}

type VerifyRequestResponse struct {
	RequestId string `json:"request_id"`
	Status    string `json:"status"`
	ErrorText string `json:"error_text"`
}

type VerifyCheckResponse struct {
	VerifyRequestResponse
	EventId  string `json:"event_id"`
	Price    string `json:"price"`
	Currency string `json:"currency"`
}

type GetTokenResponse struct {
	BaseResponse
	Token string `json:"token"`
}

// VerifyRequest needs the params map to have `number` and `brand`. `number` should be in countrycode+phonenumber format.
func (self *ClientV2) VerifyRequest(params map[string]string) (VerifyRequestResponse, error) {
	var respObj VerifyRequestResponse

	req, err := self.generateRequestV2(params, endpointUrls["verifyRequest"])
	if err != nil {
		return respObj, err
	}
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return respObj, err
	}

	defer resp.Body.Close()
	content, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		panic(err.Error())
	}

	if err := json.Unmarshal(content, &respObj); err != nil {
		return respObj, err
	}

	return respObj, nil
}

//VerifyCheck requires request_id and code to be passed as params.
func (self *ClientV2) VerifyCheck(params map[string]string) (VerifyCheckResponse, error) {
	var respObj VerifyCheckResponse

	req, err := self.generateRequestV2(params, endpointUrls["verifyCheck"])
	if err != nil {
		return respObj, err
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return respObj, err
	}

	defer resp.Body.Close()
	content, _ := ioutil.ReadAll(resp.Body)

	if err := json.Unmarshal(content, &respObj); err != nil {
		return respObj, err
	}

	return respObj, nil
}

// VerifySearch accepts a map[string]string with device_id, source_ip_address, number and country (optional)
// It returns a VerifySearchResponse struct
func (self *Client) VerifySearch(params map[string]string) (VerifySearchResponse, error) {
	var respObj VerifySearchResponse

	tokenResponse, err := self.GetToken(map[string]string{
		"device_id":         params["device_id"],
		"source_ip_address": params["source_ip_address"],
	})
	if err != nil {
		return respObj, nil
	}

	if tokenResponse.ResultCode != 0 {
		return respObj, errors.New(tokenResponse.ResultMessage)
	}

	params["token"] = tokenResponse.Token

	req, err := self.generateRequest(params, endpointUrls["verifySearch"])
	if err != nil {
		return respObj, err
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return respObj, err
	}

	defer resp.Body.Close()
	content, _ := ioutil.ReadAll(resp.Body)
	responseSignature := resp.Header.Get("X-NEXMO-RESPONSE-SIGNATURE")

	if err = validateResponseSignature(content, responseSignature, self.sharedSecret); err != nil {
		return respObj, err
	}

	if err := json.Unmarshal(content, &respObj); err != nil {
		return respObj, err
	}

	return respObj, nil
}

// GetToken accepts a map[string]string with device_id and source_ip_address.
// It returns a GetTokenResponse struct
func (self *Client) GetToken(params map[string]string) (GetTokenResponse, error) {
	var respObj GetTokenResponse

	req, err := self.generateRequest(params, endpointUrls["getToken"])
	if err != nil {
		return respObj, err
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return respObj, err
	}

	defer resp.Body.Close()
	content, _ := ioutil.ReadAll(resp.Body)
	responseSignature := resp.Header.Get("X-NEXMO-RESPONSE-SIGNATURE")

	if err = validateResponseSignature(content, responseSignature, self.sharedSecret); err != nil {
		return respObj, err
	}

	if err := json.Unmarshal(content, &respObj); err != nil {
		return respObj, err
	}
	return respObj, nil
}

func (self *Client) generateRequest(params map[string]string, url string) (*http.Request, error) {
	payload := generateParameters(params, self.appId, self.sharedSecret)
	queryString := mapToQueryString(payload)

	req, _ := http.NewRequest("GET", url+"?"+queryString, bytes.NewBuffer([]byte(``)))

	req.Header.Add("Content-Type", "application/x­www­form­urlencoded")
	req.Header.Add("Content-Encoding", "UTF-8")
	req.Header.Add("X-NEXMO-SDK-OS-FAMILY", self.osFamily)
	req.Header.Add("X-NEXMO-SDK-OS-REVISION", self.osRevision)
	req.Header.Add("X-NEXMO-SDK-REVISION", self.sdkRevision)

	return req, nil
}

func (self *ClientV2) generateRequestV2(params map[string]string, url string) (*http.Request, error) {
	payload := generateParametersV2(params, self.apiKey, self.apiSecret)
	queryString := mapToQueryString(payload)

	req, _ := http.NewRequest("GET", url+"?"+queryString, bytes.NewBuffer([]byte(``)))

	return req, nil
}

func generateParametersV2(params map[string]string, apiKey, apiSecret string) map[string]string {
	copyParams := make(map[string]string)

	for key, value := range params {
		copyParams[key] = value
	}

	copyParams["api_key"] = apiKey
	copyParams["api_secret"] = apiSecret

	return copyParams
}

func generateParameters(params map[string]string, appId, sharedSecret string) map[string]string {
	copyParams := make(map[string]string)

	for key, value := range params {
		copyParams[key] = value
	}

	copyParams["app_id"] = appId
	copyParams["timestamp"] = strconv.Itoa(int(time.Now().UTC().Unix()))
	copyParams["sig"] = createSignature(copyParams, sharedSecret)

	return copyParams
}

func createSignature(params map[string]string, sharedSecret string) string {
	sortedKeys := sortKeys(params)
	queryString := ""

	for _, key := range sortedKeys {
		value := params[key]

		if key == "token" {
			value = tokenReplaceRegexp.ReplaceAllString(value, "_")
		}

		queryString = queryString + "&" + key + "=" + value
	}

	if len(queryString) == 0 {
		return ""
	}

	queryString = queryString + sharedSecret
	signature := md5.Sum([]byte(queryString))

	return hex.EncodeToString(signature[:])
}

func mapToQueryString(params map[string]string) string {
	data := url.Values{}

	for key, value := range params {
		data.Set(key, value)
	}

	return data.Encode()
}

func sortKeys(params map[string]string) []string {
	keys := make([]string, 0)

	for key, _ := range params {
		keys = append(keys, key)
	}

	sort.Strings(keys)

	return keys
}

func validateResponseSignature(responseJson []byte, responseSignature, sharedSecret string) error {
	if len(responseSignature) == 0 {
		return errors.New("Missing Response Signature")
	}

	signatureString := string(responseJson) + sharedSecret
	signature := md5.Sum([]byte(signatureString))

	if hex.EncodeToString(signature[:]) != responseSignature {
		return errors.New("Invalid Response Signature")
	}

	return nil
}
