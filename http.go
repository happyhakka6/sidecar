package main

import (
	"encoding/json"
	"html/template"
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	"github.com/hashicorp/memberlist"
	"github.com/newrelic/bosun/services_state"
)


func makeHandler(fn func (http.ResponseWriter, *http.Request, *memberlist.Memberlist, *services_state.ServicesState), list *memberlist.Memberlist, state *services_state.ServicesState) http.HandlerFunc {
	return func(response http.ResponseWriter, req *http.Request) {
		fn(response, req, list, state)
	}
}

func servicesHandler(response http.ResponseWriter, req *http.Request, list *memberlist.Memberlist, state *services_state.ServicesState) {
	params := mux.Vars(req)

	defer req.Body.Close()

	if params["extension"] == ".json" {
		response.Header().Set("Content-Type", "application/json")
		jsonStr, _ := json.MarshalIndent(state.ByService(), "", "  ")
		response.Write(jsonStr)
		return
	}

	response.Header().Set("Content-Type", "text/html")
	response.Write(
		[]byte(`
 			<head>
 			<meta http-equiv="refresh" content="4">
 			</head> 
	    	<pre>` + state.Format(list) + "</pre>"))
}

func statusStr(status int) string {
	switch status {
	case 0:
		return "Alive"
	case 1:
		return "Tombstone"
	default:
		return ""
	}
}

func TimeAgo(when time.Time, ref time.Time) string {
	diff := ref.Round(time.Second).Sub(when.Round(time.Second))

	switch {
	case diff > time.Hour * 24 * 7:
	    result := diff.Hours() / 24 / 7
		return strconv.FormatFloat(result, 'f', 1, 64) + " weeks ago"
	case diff > time.Hour * 24:
	    result := diff.Hours() / 24
		return strconv.FormatFloat(result, 'f', 1, 64) + " days ago"
	case diff > time.Hour:
	    result := diff.Hours()
		return strconv.FormatFloat(result, 'f', 1, 64) + " hours ago"
	case diff > time.Minute:
		result := diff.Minutes()
		return strconv.FormatFloat(result, 'f', 1, 64) + " mins ago"
	case diff > time.Second:
		return strconv.FormatFloat(diff.Seconds(), 'f', 1, 64) + " secs ago"
	default:
		return "1.0 sec ago"
	}
}

type listByName []*memberlist.Node
func (a listByName) Len() int           { return len(a) }
func (a listByName) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a listByName) Less(i, j int) bool { return a[i].Name < a[j].Name }

func viewHandler(response http.ResponseWriter, req *http.Request, list *memberlist.Memberlist, state *services_state.ServicesState) {
	timeAgo := func(when time.Time) string { return TimeAgo(when, time.Now().UTC()) }

	funcMap := template.FuncMap{"statusStr": statusStr, "timeAgo": timeAgo}
	t, err := template.New("services").Funcs(funcMap).ParseFiles("views/services.html")
	if err != nil {
		println("Error parsing template: " + err.Error())
	}

	members := list.Members()
	sort.Sort(listByName(members))

	viewData := map[string]interface{}{
		"Services": state.ByService(),
		"Members":  members,
	}

	t.ExecuteTemplate(response, "services.html", viewData)
}


func serveHttp(list *memberlist.Memberlist, state *services_state.ServicesState) {
	router := mux.NewRouter()

	router.HandleFunc(
		"/services{extension}", makeHandler(servicesHandler, list, state),
	).Methods("GET")

	router.HandleFunc(
		"/servers", makeHandler(servicesHandler, list, state),
	).Methods("GET")

	router.HandleFunc(
		"/services", makeHandler(viewHandler, list, state),
	).Methods("GET")

	http.Handle("/", router)

	err := http.ListenAndServe("0.0.0.0:7777", nil)
	exitWithError(err, "Can't start HTTP server")
}
