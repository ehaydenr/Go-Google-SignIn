package main

import (
	"encoding/json"
	"github.com/gorilla/sessions"
	"html/template"
	"io/ioutil"
	"net/http"
	"os"
)

var client_id string = func() string {
	if s := os.Getenv("client_id"); s != "" {
		return s
	}
	return "YOUR-CLIENT-ID"
}()
var session_name string = "mysession"
var session_secret string = "something-secret"

var sign_in_template, _ = template.New("sign_in_template").Parse(`
<html lang="en">
  <head>
    <meta name="google-signin-scope" content="profile email">
    <meta name="google-signin-client_id" content="{{.}}">
    <script src="https://apis.google.com/js/platform.js" async defer></script>
  </head>
  <body>
    <div class="g-signin2" data-onsuccess="onSignIn" data-theme="dark"></div>
		<script src="https://code.jquery.com/jquery-2.1.4.js"></script>
    <script>
      function onSignIn(googleUser) {
        var id_token = googleUser.getAuthResponse().id_token;
				$.post('/oauth', {token: id_token}, function(){
					location.reload();
				});
      };
    </script>
  </body>
</html>
`)

var main_page, _ = template.New("main_page").Parse(`
<html>
	<head></head>
	<body>Hello {{.Name}} - {{.Id}}</body>
</html>
`)

type User struct {
	Name string
	Id   string
}

var store = sessions.NewCookieStore([]byte(session_secret))

func root(w http.ResponseWriter, r *http.Request, u *User) {
	main_page.Execute(w, u)
}

func oauthCallback(w http.ResponseWriter, r *http.Request) {
	session, err := store.Get(r, session_name)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	old_token := session.Values["token"]

	token := r.FormValue("token")

	if token == "" && old_token != nil { // No new token, but old one present
		http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
	} else if token == "" { // No token at all
		sign_in_template.Execute(w, client_id)
	} else { // New Token!
		session.Values["token"] = string(token)
		session.Save(r, w)
	}
}

func makeSecureHandler(fn func(http.ResponseWriter, *http.Request, *User)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session, err := store.Get(r, session_name)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		token_interface := session.Values["token"]
		if token_interface == nil {
			http.Redirect(w, r, "/oauth", http.StatusTemporaryRedirect)
			return
		}

		token, ok := token_interface.(string)
		if !ok {
			session.Values["token"] = nil
			http.Redirect(w, r, "/oauth", http.StatusTemporaryRedirect)
			return
		}

		res, _ := http.Get("https://www.googleapis.com/oauth2/v3/tokeninfo?id_token=" + token)
		defer res.Body.Close()
		body, _ := ioutil.ReadAll(res.Body)

		var m map[string]string
		json.Unmarshal(body, &m)
		u := &User{
			Name: m["name"],
			Id:   m["sub"],
		}

		fn(w, r, u)
	}
}

func main() {
	http.HandleFunc("/", makeSecureHandler(root))
	http.HandleFunc("/oauth", oauthCallback)
	http.ListenAndServe(":8080", nil)
}
