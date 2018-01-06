package gc

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type GarminConnect interface {
	Login() error
	NextActivity() *Activity
	ExportTCX(activityID int64) ([]byte, error)
}

const userAgent = "Mozilla/5.0 (X11; Linux x86_64; rv:57.0) Gecko/20100101 Firefox/57.0"
const ssoURLStr = "https://sso.garmin.com/sso/login?service=https://connect.garmin.com/modern/&webhost=https://connect.garmin.com&source=https://connect.garmin.com/en-US/signin&redirectAfterAccountLoginUrl=https://connect.garmin.com/modern/&redirectAfterAccountCreationUrl=https://connect.garmin.com/modern/&gauthHost=https://sso.garmin.com/sso&locale=en_US&id=gauth-widget&cssUrl=https://static.garmincdn.com/com.garmin.connect/ui/css/gauth-custom-v1.2-min.css&privacyStatementUrl=//connect.garmin.com/en-US/privacy/&clientId=GarminConnect&rememberMeShown=true&rememberMeChecked=false&createAccountShown=true&openCreateAccount=false&displayNameShown=false&consumeServiceTicket=false&initialFocus=true&embedWidget=false&generateExtraServiceTicket=false&globalOptInShown=true&globalOptInChecked=false&mobile=false&connectLegalTerms=true"
const legacySessionURLStr = "https://connect.garmin.com/legacy/session"
const activitySearchURLStr = "https://connect.garmin.com/proxy/activity-search-service-1.2/json/activities"
const exportTCXURLStr = "https://connect.garmin.com/modern/proxy/download-service/export/tcx/activity/%d"

var responseURLRegex = regexp.MustCompile(`\bresponse_url\s*=\s*"([^"]*)"`)

type activitiesPage struct {
	Results results `json:"results"`
}

type results struct {
	Activities []gcActivityWrapper `json:"activities"`
	TotalFound int                 `json:"totalFound"`
}

type gcActivityWrapper struct {
	Activity gcActivity `json:"activity"`
}

type gcActivity struct {
	ID         int64                 `json:"activityId"`
	Name       string                `json:"activityName"`
	UploadDate gregorianCalendarTime `json:"uploadDate"`
}

type gregorianCalendarTime struct {
	Millis string `json:"millis"`
}

func (g gregorianCalendarTime) goTime() time.Time {
	millis, _ := strconv.ParseInt(g.Millis, 10, 64)
	return time.Unix(0, millis*int64(time.Millisecond))
}

type gcCookieJar struct {
	http.CookieJar
}

func newCookieJar(o *cookiejar.Options) (*gcCookieJar, error) {
	wrapped, err := cookiejar.New(o)
	if err != nil {
		return nil, err
	}
	return &gcCookieJar{wrapped}, nil
}

func (jar *gcCookieJar) SetCookies(u *url.URL, cookies []*http.Cookie) {
	/*for _, cookie := range cookies {
		if cookie.Name == "JSESSIONID" {
			cookie.Path = "/"
			cookie.Domain = "garmin.com"
		}
	}*/
	jar.CookieJar.SetCookies(u, cookies)
}

type garminConnectImpl struct {
	username        string
	password        string
	client          *http.Client
	activities      []Activity
	activityCounter int
}

func NewGarminConnect(username, password string) GarminConnect {
	return &garminConnectImpl{username: username, password: password}
}

func (gc *garminConnectImpl) Login() error {
	// 1st do a GET on the SSO URL to get a session cookie
	cookieJar, _ := newCookieJar(nil)
	gc.client = &http.Client{Timeout: 10 * time.Second, Jar: cookieJar}
	if err := gc.loadPage(ssoURLStr); err != nil {
		return err
	}

	form := url.Values{}
	form.Set("username", gc.username)
	form.Add("password", gc.password)
	form.Add("embed", "false")
	request, err := http.NewRequest("POST", ssoURLStr,
		strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	request.Header.Set("User-Agent", userAgent)
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := gc.client.Do(request)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	// Look for response_url = "<url>/?ticket=...";
	responseURL, err := getResponseURL(body)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Login: unexpected status code: %d", resp.StatusCode)
	}

	// Go here to get some session cookies
	if err = gc.loadPage(responseURL); err != nil {
		return err
	}

	// Go here to get JSESSIONID cookie
	if err = gc.loadPage(legacySessionURLStr); err != nil {
		return err
	}

	//body, err = ioutil.ReadAll(resp.Body)
	//fmt.Println(string(body))

	return gc.getActivities()
}

func (gc *garminConnectImpl) loadPage(url string) error {
	request, err := http.NewRequest("GET", url, nil)
	request.Header.Set("User-Agent", userAgent)
	resp, err := gc.client.Do(request)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		msg, _ := ioutil.ReadAll(resp.Body)
		return fmt.Errorf("Login: unexpected status code: %d: %s", resp.StatusCode, msg)
	}
	return nil
}

func getResponseURL(body []byte) (string, error) {
	matches := responseURLRegex.FindSubmatch(body)
	if matches == nil {
		return "", fmt.Errorf("response URL not found - login probably failed")
	}
	result := string(matches[1])
	result = strings.Replace(result, "\\", "", -1)
	return result, nil
}

func getTGTCookie(jar *cookiejar.Jar) (string, error) {
	ssoURL, _ := url.Parse(ssoURLStr)
	cookies := jar.Cookies(ssoURL)
	for _, cookie := range cookies {
		if cookie.Name == "CASTGC" {
			return cookie.Value, nil
		}
	}
	return "", fmt.Errorf("Did not get a CASTGC cookie - login probably failed")
}

func (gc *garminConnectImpl) getActivities() error {
	request, err := http.NewRequest("GET", activitySearchURLStr, nil)
	request.Header.Set("User-Agent", userAgent)
	resp, err := gc.client.Do(request)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		msg, _ := ioutil.ReadAll(resp.Body)
		return fmt.Errorf("Activity search: unexpected status code : %d: %s", resp.StatusCode, msg)
	}
	decoder := json.NewDecoder(resp.Body)
	var activitiesPage activitiesPage
	err = decoder.Decode(&activitiesPage)
	if err != nil {
		return err
	}
	gc.activities = make([]Activity, len(activitiesPage.Results.Activities))
	idx := 0
	for _, gcActivityWrapper := range activitiesPage.Results.Activities {
		gcActivity := gcActivityWrapper.Activity
		gc.activities[idx] = Activity{ID: gcActivity.ID,
			Name: gcActivity.Name, UploadDate: gcActivity.UploadDate.goTime()}
		idx++
	}
	return nil
}

func (gc *garminConnectImpl) NextActivity() *Activity {
	if len(gc.activities) > gc.activityCounter {
		result := &gc.activities[gc.activityCounter]
		gc.activityCounter++
		return result
	}
	return nil
}

func (gc garminConnectImpl) ExportTCX(activityID int64) ([]byte, error) {
	exportURL := fmt.Sprintf(exportTCXURLStr, activityID)
	request, err := http.NewRequest("GET", exportURL, nil)
	request.Header.Set("User-Agent", userAgent)
	resp, err := gc.client.Do(request)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		msg, _ := ioutil.ReadAll(resp.Body)
		return nil, fmt.Errorf("Export TCX: unexpected status code : %d: %s", resp.StatusCode, msg)
	}
	return ioutil.ReadAll(resp.Body)
}
