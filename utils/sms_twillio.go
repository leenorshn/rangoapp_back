package utils

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/kevinburke/twilio-go"
)

var (
	accountSid = "ACce339b854f4bda3f03d64bc828efb01a"
	authToken  = "c82adfc7ca8fb11a8625c83175e13471"
)

func SendMessageSMS(to string, message string) bool {
	client := twilio.NewClient(accountSid, authToken, nil)

	msg, err := client.Messages.SendMessage("innov", to, message, nil)
	if err != nil {
		fmt.Errorf("%s", err)
		return false
	}

	fmt.Println(msg.Status)
	return true

}
func GenerateCode() int {
	rand.Seed(time.Now().UnixNano())
	code := rand.Intn(1000000)
	return code
}
