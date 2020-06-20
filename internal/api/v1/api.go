package v1

import (
    "io"
	"net/http"
    "log"
	"fmt"
    "crypto/sha1"
    "encoding/hex"
    "regexp"
    "time"
	"strconv"
	"errors"
    "io/ioutil"
	"encoding/json"
	"github.com/ltkh/alertstrap/internal/db"
	"github.com/ltkh/alertstrap/internal/cache"
	"github.com/ltkh/alertstrap/internal/config"
	"github.com/ltkh/alertstrap/internal/ldap"
)

var (
	CacheAlerts *cache.Alerts = cache.NewCacheAlerts()
	CacheUsers *cache.Users = cache.NewCacheUsers()
	re_labels = regexp.MustCompile(`(?:([\w]+)([=!~]{1,2})"([^"]*)")`)
)

type Api struct {
	Client       db.DbClient
	Conf         *config.Config
}

type Resp struct {
	Status       string                  `json:"status"`
	Error        string                  `json:"error,omitempty"`
	Warnings     []string                `json:"warnings,omitempty"`
	Data         interface{}             `json:"data"`
}

type Alerts struct {
	Position     int64                   `json:"position"`
  	AlertsArray  []Alert                 `json:"alerts"`
}

type Alert struct {
  	AlertId      string                  `json:"alertId"`
  	GroupId      string                  `json:"groupId"`
	Status       string                  `json:"status"`
  	StartsAt     time.Time               `json:"startsAt"`
  	EndsAt       time.Time               `json:"endsAt"`
	Repeat       int                     `json:"repeat"`
	ChangeSt     int                     `json:"changeSt"`
  	Labels       map[string]interface{}  `json:"labels"`
  	Annotations  map[string]interface{}  `json:"annotations"`
  	GeneratorURL string                  `json:"generatorURL"`
}

// Matcher models the matching of a label.
type Matcher struct {
	Type  string
	Name  string
	Value string
	re *regexp.Regexp
}

// NewMatcher returns a matcher object.
func newMatcher(t, n, v string) (*Matcher, error) {
	m := &Matcher{
		Type:  t,
		Name:  n,
		Value: v,
	}
	if t == "=~" || t == "!~" {
		re, err := regexp.Compile("^(?:" + v + ")$")
		if err != nil {
			return nil, err
		}
		m.re = re
	}
	return m, nil
}

// Matches returns whether the matcher matches the given string value.
func (m *Matcher) matches(s string) bool {
	switch m.Type {
	case "=":
		return s == m.Value
	case "!=":
		return s != m.Value
	case "=~":
		return m.re.MatchString(s)
	case "!~":
		return !m.re.MatchString(s)
	}
	return false
}

func encodeResp(resp *Resp) []byte {
    jsn, err := json.Marshal(resp)
	if err != nil {
		return encodeResp(&Resp{Status:"error", Error:err.Error(), Data:make(map[string]string, 0)})
	}
	return jsn
}

func getHash(text string) string {
	h := sha1.New()
	io.WriteString(h, text)
	return hex.EncodeToString(h.Sum(nil))
}

func parseMetricSelector(input string) (m []*Matcher, err error) {
	var matchers []*Matcher

	lbls := re_labels.FindAllStringSubmatch(input, -1)
	for _, l := range lbls {

		matcher, err := newMatcher(l[2], l[1], l[3])
		if err != nil {
			return nil, err
		}

		matchers = append(matchers, matcher)
	}

	return matchers, nil
}

func checkMatch(alert *cache.Alert, matchers [][]*Matcher) bool {
	for _, mtch := range matchers {
        match := true

		for _, m := range mtch {
			val := alert.Labels[m.Name]
			if val == nil {
                val = ""
			}
			if !m.matches(val.(string)) {
				match = false
			    break
			}
		}

		if match {
			return true
		}
	}

	return false
}

func authentication(cln db.DbClient, cfg config.DB, r *http.Request) (bool, int, error) {
	var login, token string

	login, token, ok := r.BasicAuth()
    if !ok {
		lg, err := r.Cookie("login")
		if err == nil {
			login = lg.Value
		}
		tk, err := r.Cookie("token")
		if err == nil {
			token = tk.Value
		}
	}

	if login != "" && token != "" {
		user, ok := CacheUsers.Get(login)
		if !ok { 
			usr, err := cln.LoadUser(login)
			if err != nil {
				return false, 403, errors.New("Forbidden")
			}
			CacheUsers.Set(login, usr)
			if usr.Token == token {
				return true, 204, nil
			}
			return false, 403, errors.New("Forbidden")
		} else {
			if user.Token == token {
				return true, 204, nil
			}
			return false, 403, errors.New("Forbidden")
		}
	}

	return false, 401, errors.New("Unauthorized")
}

func New(conf *config.Config) (*Api, error) {
	//connection to data base
	client, err := db.NewClient(&conf.DB)
	if err != nil {
		return nil, err
	}
	log.Print("[info] connected to dbase")
	//loading alerts
	alerts, err := client.LoadAlerts()
	if err != nil {
		return nil, err
	}
	for _, alert := range alerts {
		CacheAlerts.Set(alert.GroupId, alert)
	}
	log.Printf("[info] loaded alerts from dbase (%d)", len(alerts))
	//loading users
	users, err := client.LoadUsers()
	if err != nil {
		return nil, err
	}
	for _, user := range users {
		CacheUsers.Set(user.Login, user)
	}
	log.Printf("[info] loaded users from dbase (%d)", len(users))
	
	return &Api{ Client: client, Conf: conf }, nil
}

func (api *Api) ApiHealthy(w http.ResponseWriter, r *http.Request) {
	var alerts []string

	if err := api.Client.Healthy(); err != nil {
        alerts = append(alerts, err.Error())
	}

	if len(alerts) > 0 {
		w.WriteHeader(200)
		w.Write(encodeResp(&Resp{Status:"success", Warnings:alerts, Data:make(map[string]string, 0)}))
		return
	}

    w.WriteHeader(200)
	w.Write(encodeResp(&Resp{Status:"success", Data:make(map[string]string, 0)}))
}

func (api *Api) ApiAuth(w http.ResponseWriter, r *http.Request) {
    ok, code, err := authentication(api.Client, api.Conf.DB, r)
	if !ok {
		w.WriteHeader(code)
		w.Write(encodeResp(&Resp{Status:"error", Error:err.Error(), Data:make(map[string]string, 0)}))
		return
	}
	w.WriteHeader(code)
	w.Write(encodeResp(&Resp{Status:"success", Data:make(map[string]string, 0)}))
}

func (api *Api) ApiMenu(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	/*
	jsn, err := json.Marshal(api.Menu)
	if err != nil {
		log.Printf("[error] %v - %s", err, r.URL.Path)
		w.WriteHeader(400)
		w.Write(encodeResp(&Resp{Status:"error", Error:err.Error()}))
		return
	}
	*/

	//log.Printf("[error] %q", api.Menu)
	
	//var nodes Nodes;
	//for _, m := range api.Menu {
	//	for _, v := range m.Section {
	//		log.Printf("%v - %v", m.Name, v.Name)
	//	}
	//}
	w.Write(encodeResp(&Resp{Status:"success", Data:api.Conf.Menu}))
}

func (api *Api) ApiAlerts(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method == "GET" {

		var alerts Alerts

		ok, code, err := authentication(api.Client, api.Conf.DB, r)
		if !ok {
			w.WriteHeader(code)
			w.Write(encodeResp(&Resp{Status:"error", Error:err.Error(), Data:make(map[string]string, 0)}))
			return
		}

		//limit setting
		limit := api.Conf.Server.Alerts_limit
		if r.URL.Query()["limit"] !=nil {
			l, err := strconv.Atoi(r.URL.Query()["limit"][0])
			if err == nil && l < limit {
                limit = l
			}
		}

		//match setting
		var matcherSets [][]*Matcher
		for _, s := range r.URL.Query()["match[]"] {
			matchers, err := parseMetricSelector(s)
			if err != nil {
				log.Printf("[error] %v - %s", err, r.URL.Path)
				w.WriteHeader(400)
				w.Write(encodeResp(&Resp{Status:"error", Error:err.Error(), Data:make(map[string]string, 0)}))
				return
			}
			matcherSets = append(matcherSets, matchers)
		}

        //position settings
        position := int64(0)
		if r.URL.Query()["position"] != nil {
			i, err := strconv.Atoi(r.URL.Query()["position"][0])
			if err == nil {
				position = int64(i) 
			}
		}

		//status settings
		var re_status *regexp.Regexp
		if r.URL.Query()["status"] != nil {
			re, err := regexp.Compile("^(?:" + r.URL.Query()["status"][0] + ")$")
			if err != nil {
				log.Printf("[error] %v - %s", err, r.URL.Path)
				w.WriteHeader(400)
				w.Write(encodeResp(&Resp{Status:"error", Error:err.Error(), Data:make(map[string]string, 0)}))
				return 
			}
			re_status = re
		}

		for _, a := range CacheAlerts.Items() {

            if position != 0 && a.ActiveAt < position {
                continue
			}
			if re_status != nil && !re_status.MatchString(a.Status) {
                continue
			}
			if len(matcherSets) != 0 && !checkMatch(&a, matcherSets) {
                continue
			}

			var alert Alert

			alert.AlertId      = a.AlertId
			alert.GroupId      = a.GroupId
			alert.Status       = a.Status
			alert.StartsAt     = time.Unix(a.StartsAt, 0)
			alert.EndsAt       = time.Unix(a.EndsAt, 0)
			alert.Repeat       = a.Repeat
			alert.ChangeSt     = a.ChangeSt
			alert.Labels       = a.Labels
			alert.Annotations  = a.Annotations
			alert.GeneratorURL = a.GeneratorURL

			alerts.AlertsArray = append(alerts.AlertsArray, alert)

			if a.ActiveAt > alerts.Position {
				alerts.Position  = a.ActiveAt
			}
			
			if len(alerts.AlertsArray) >= limit {
				var warnings []string
				if limit == api.Conf.Server.Alerts_limit {
					warnings = append(warnings, fmt.Sprintf("display limit exceeded - %d", limit))
				}
				w.Write(encodeResp(&Resp{Status:"success", Warnings:warnings, Data:alerts}))
				return
			}
		}

		if len(alerts.AlertsArray) == 0 {
			alerts.AlertsArray = make([]Alert, 0)
		}
		
		w.Write(encodeResp(&Resp{Status:"success", Data:alerts}))
		return
	}

    if r.Method == "POST" {

		var alerts []Alert

		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			log.Printf("[error] %v - %s", err, r.URL.Path)
			w.WriteHeader(400)
			w.Write(encodeResp(&Resp{Status:"error", Error:err.Error(), Data:make(map[string]string, 0)}))
			return
		}

		if err := json.Unmarshal(body, &alerts); err != nil {
			log.Printf("[error] %v - %s", err, r.URL.Path)
			w.WriteHeader(400)
			w.Write(encodeResp(&Resp{Status:"error", Error:err.Error(), Data:make(map[string]string, 0)}))
			return
		}

		go func(data []Alert){

			for _, value := range data {

				labels, err := json.Marshal(value.Labels)
				if err != nil {
					log.Printf("[error] read alert %v", err)
					return
				}

				if value.Status == "" {
                    value.Status = "firing"
				}
			
				starts_at := value.StartsAt.UTC().Unix()
				ends_at   := value.EndsAt.UTC().Unix()
				if starts_at < 0 {
					starts_at  = time.Now().UTC().Unix()
				} 
				if ends_at < 0 {
					ends_at    = time.Now().UTC().Unix() + api.Conf.Server.Alerts_resolve
				} 
			
				group_id := getHash(string(labels))
			
				alert, found := CacheAlerts.Get(group_id)
				if found {

					if alert.Status != value.Status {
						alert.ChangeSt ++ 
					}

					alert.Status         = value.Status
					alert.ActiveAt       = time.Now().UTC().Unix()
					alert.StartsAt       = starts_at
					alert.EndsAt         = ends_at
					alert.Annotations    = value.Annotations
					alert.GeneratorURL   = value.GeneratorURL
					alert.Repeat         = alert.Repeat + 1
			
					CacheAlerts.Set(group_id, alert)
			
				} else {

					alert_id := getHash(string(strconv.FormatInt(time.Now().UTC().UnixNano(), 16)+group_id))
					
					var alert cache.Alert

					alert.AlertId        = alert_id
					alert.GroupId        = group_id
					alert.Status         = value.Status
					alert.ActiveAt       = time.Now().UTC().Unix()
					alert.StartsAt       = starts_at
					alert.EndsAt         = ends_at
					alert.Labels         = value.Labels
					alert.Annotations    = value.Annotations
					alert.GeneratorURL   = value.GeneratorURL
					alert.Repeat         = 1
					alert.ChangeSt       = 0
			
					CacheAlerts.Set(group_id, alert)

				}

			}

		}(alerts)

		w.WriteHeader(204)
		return
	}

	if r.Method == "DELETE" {
        ok, code, err := authentication(api.Client, api.Conf.DB, r)
		if !ok {
			w.WriteHeader(code)
			w.Write(encodeResp(&Resp{Status:"error", Error:err.Error(), Data:make(map[string]string, 0)}))
			return
		}

		if r.URL.Query()["groupId"] != nil {
			
			_, found := CacheAlerts.Get(r.URL.Query()["groupId"][0])
			if found {
				CacheAlerts.Delete(r.URL.Query()["groupId"][0])
                w.WriteHeader(200)
				w.Write(encodeResp(&Resp{Status:"success", Data:make(map[string]string, 0)}))
				return
			}

			w.WriteHeader(400)
			w.Write(encodeResp(&Resp{Status:"error", Error:"Alert Not Found", Data:make(map[string]string, 0)}))
			return

		}

		w.WriteHeader(400)
		w.Write(encodeResp(&Resp{Status:"error", Error:"GroupId required", Data:make(map[string]string, 0)}))
		return
	}

	w.WriteHeader(405)
	w.Write(encodeResp(&Resp{Status:"error", Error:"Method Not Allowed", Data:make(map[string]string, 0)}))
}

func (api *Api) ApiLogin(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	err := r.ParseForm()
	if err != nil {
		w.WriteHeader(400)
		w.Write(encodeResp(&Resp{Status:"error", Error:err.Error(), Data:make(map[string]string, 0)}))
		return
	}

	r.ParseForm()
	username := r.Form.Get("login")
	password := r.Form.Get("password")

	if username == "" || password == "" {
		w.WriteHeader(403)
		w.Write(encodeResp(&Resp{Status:"error", Error:"Login or password is empty", Data:make(map[string]string, 0)}))
		return
	}

	if api.Conf.Ldap.Bind_user == "" && api.Conf.Ldap.Bind_pass == "" {
		api.Conf.Ldap.Bind_user = username
		api.Conf.Ldap.Bind_pass = password
	}

	var attributes []string
	for _, val := range api.Conf.Ldap.Attributes {
		attributes = append(attributes, val)
	}

	clnt := &ldap.LDAPClient{
		Base:         api.Conf.Ldap.Search_base,
		Host:         api.Conf.Ldap.Host,
		Port:         api.Conf.Ldap.Port,
		UseSSL:       api.Conf.Ldap.Use_ssl,
		BindDN:       fmt.Sprintf(api.Conf.Ldap.Bind_dn, api.Conf.Ldap.Bind_user),
		BindPassword: api.Conf.Ldap.Bind_pass,
		UserFilter:   api.Conf.Ldap.User_filter,
		Attributes:   attributes,
	}
	defer clnt.Close()

	ok, usr, err := clnt.Authenticate(username, password)
	if !ok {
		log.Printf("[error] user authenticating %s: %+v", username, err)
		w.WriteHeader(403)
		w.Write(encodeResp(&Resp{Status:"error", Error:err.Error(), Data:make(map[string]string, 0)}))
		return
	}

	var user cache.User
	user.Login = username
	user.Password = getHash(password)
	user.Token = getHash(string(time.Now().UTC().Unix()))
	if api.Conf.Ldap.Attributes["name"] != "" {
		user.Name = usr[api.Conf.Ldap.Attributes["name"]]
	}
	if api.Conf.Ldap.Attributes["email"] != "" {
		user.Email = usr[api.Conf.Ldap.Attributes["email"]]
	}
	
	CacheUsers.Set(username, user)

	if err := api.Client.SaveUser(user); err != nil {
		log.Printf("[error] saving user %s: %+v", username, err)
		w.WriteHeader(500)
		w.Write(encodeResp(&Resp{Status:"error", Error:err.Error(), Data:make(map[string]string, 0)}))
	}

	w.WriteHeader(200)
	w.Write(encodeResp(&Resp{Status:"success", Data:user}))
	return

}
