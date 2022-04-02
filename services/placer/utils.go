package placer

import (
	"encoding/json"
	"strings"
)

const (
	BET  = "b"
	HEAL = "h"
)

func symbolFromMarket(m string) string {
	split := strings.Split(m, "/")
	return split[0]
}

type ClientID struct {
	ID   int64  `json:"i"`
	Side string `json:"s"`
	Try  int64  `json:"t"`
}

func marshalClientID(c ClientID) string {
	data, _ := json.Marshal(c)
	return string(data)
}
func unmarshalClientID(c string) (clientID ClientID, err error) {
	err = json.Unmarshal([]byte(c), &clientID)
	return
}
