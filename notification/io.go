package notification

import (
	"encoding/json"
	"net/http"
)

type User struct {
	NetId string
	Area  string
}

type Response struct {
	Status string      `json:status`
	Data   interface{} `json:data`
}

func write(code int, res Response, w http.ResponseWriter) {
	b, err := json.Marshal(res)
	if err != nil {
		w.WriteHeader(500)
		return
	}
	w.Header().Add("Access-Control-Allow-Origin", "*")
	w.WriteHeader(code)
	w.Write(b)
}
