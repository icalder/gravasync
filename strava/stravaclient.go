package strava

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Strava interface {
	SetAccessToken(accessToken string)
	Authorise(clientID, clientSecret string) error
	ImportTCX(activityName string, private bool, tcxBytes []byte) error
	TopActivity() (*Activity, error)
}

const userAgent = "Mozilla/5.0 (X11; Linux x86_64; rv:57.0) Gecko/20100101 Firefox/57.0"
const oauthAuthorizeURLStr = "https://www.strava.com/oauth/authorize?client_id=%s&response_type=code&redirect_uri=http://localhost:8001/callback&scope=write"
const oauthTokenExchangeURLStr = "https://www.strava.com/oauth/token"
const activitiesURLStr = "https://www.strava.com/api/v3/athlete/activities"
const uploadsURLStr = "https://www.strava.com/api/v3/uploads"

type tokenResponse struct {
	AccessToken string `json:"access_token"`
}

type uploadResponse struct {
	ID         int    `json:"id"`
	ActivityID int    `json:"activity_id"`
	Status     string `json:"status"`
	Error      string `json:"error"`
}

type stravaImpl struct {
	accessToken string
	client      *http.Client
}

func NewStrava() Strava {
	result := stravaImpl{}
	result.client = &http.Client{Timeout: 10 * time.Second}
	return &result
}

func (s *stravaImpl) SetAccessToken(accessToken string) {
	s.accessToken = accessToken
}

func (s *stravaImpl) Authorise(clientID, clientSecret string) error {
	codeChannel := make(chan string)
	s.startHTTPServer(codeChannel)
	browserURL := fmt.Sprintf(oauthAuthorizeURLStr, clientID)
	fmt.Printf("Visit this URL in a browser: %s\n", browserURL)
	// http://localhost:8001/?state=&code=a600f604ea6c9c15e39a59128db927096b2c7c64
	select {
	case code := <-codeChannel:
		return s.tokenExchange(code, clientID, clientSecret)
	}
}

func (s stravaImpl) startHTTPServer(codeChannel chan<- string) {
	http.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		fmt.Fprintf(w, "Strava OAuth callback code received, please follow instructions in the console!")
		codeChannel <- code
	})

	go func() {
		err := http.ListenAndServe(":8001", nil)
		if err != nil {
			panic("ListenAndServe: " + err.Error())
		}
	}()
}

func (s *stravaImpl) tokenExchange(code, clientID, clientSecret string) error {
	form := url.Values{}
	form.Set("client_id", clientID)
	form.Set("client_secret", clientSecret)
	form.Set("code", code)
	request, err := http.NewRequest("POST", oauthTokenExchangeURLStr,
		strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	request.Header.Set("User-Agent", userAgent)
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := s.client.Do(request)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		msg, _ := ioutil.ReadAll(resp.Body)
		return fmt.Errorf("Token exchange: unexpected status code : %d: %s", resp.StatusCode, msg)
	}
	decoder := json.NewDecoder(resp.Body)
	var tokenResponse tokenResponse
	err = decoder.Decode(&tokenResponse)
	if err != nil {
		return err
	}
	s.accessToken = tokenResponse.AccessToken
	fmt.Printf("Set strava.accessToken: %s in ~/.gravasync\n", tokenResponse.AccessToken)
	return nil
}

func (s stravaImpl) TopActivity() (*Activity, error) {
	params := url.Values{}
	params.Set("per_page", "1")
	activitiesURL, err := url.Parse(activitiesURLStr)
	if err != nil {
		return nil, err
	}
	activitiesURL.RawQuery = params.Encode()

	request, err := http.NewRequest("GET", activitiesURL.String(), nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("User-Agent", userAgent)
	request.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.accessToken))
	resp, err := s.client.Do(request)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		msg, _ := ioutil.ReadAll(resp.Body)
		return nil, fmt.Errorf("GET activities: unexpected status code : %d: %s", resp.StatusCode, msg)
	}
	decoder := json.NewDecoder(resp.Body)
	var activities []Activity
	err = decoder.Decode(&activities)
	if err != nil {
		return nil, err
	}
	if len(activities) > 0 {
		return &activities[0], nil
	}
	return nil, nil
}

func (s stravaImpl) ImportTCX(activityName string, private bool, tcxBytes []byte) error {
	var b bytes.Buffer
	form := multipart.NewWriter(&b)
	// https://stackoverflow.com/questions/20205796/golang-post-data-using-the-content-type-multipart-form-data
	field, err := form.CreateFormFile("file", "activity.tcx")
	if err != nil {
		return err
	}
	if _, err = io.Copy(field, bytes.NewReader(tcxBytes)); err != nil {
		return err
	}
	// http://strava.github.io/api/v3/uploads/
	if err = s.addMultipartField(form, "data_type", "tcx"); err != nil {
		return err
	}
	if err = s.addMultipartField(form, "name", activityName); err != nil {
		return err
	}
	if private {
		if err = s.addMultipartField(form, "private", "1"); err != nil {
			return err
		}
	}
	form.Close()

	request, err := http.NewRequest("POST", uploadsURLStr, &b)
	if err != nil {
		return err
	}
	request.Header.Set("User-Agent", userAgent)
	request.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.accessToken))
	request.Header.Set("Content-Type", form.FormDataContentType())
	resp, err := s.client.Do(request)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		msg, _ := ioutil.ReadAll(resp.Body)
		return fmt.Errorf("Upload activity: unexpected status code : %d: %s", resp.StatusCode, msg)
	}
	decoder := json.NewDecoder(resp.Body)
	var uploadResponse uploadResponse
	err = decoder.Decode(&uploadResponse)
	if err != nil {
		return err
	}
	if uploadResponse.Error != "" {
		return fmt.Errorf(uploadResponse.Error)
	}
	fmt.Println(uploadResponse.Status)
	return nil
}

func (s stravaImpl) addMultipartField(form *multipart.Writer, fieldName, value string) error {
	field, err := form.CreateFormField(fieldName)
	if err != nil {
		return err
	}
	_, err = field.Write([]byte(value))
	return err
}
