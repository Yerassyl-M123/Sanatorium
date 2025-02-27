package services

import (
	"fmt"

	"github.com/sfreiberg/gotwilio"
)

const (
	accountSID  = ""
	authToken   = ""
	twilioPhone = ""
)

func SendSMS(phone string, message string) {
	twilio := gotwilio.NewTwilioClient(accountSID, authToken)
	_, _, err := twilio.SendSMS(twilioPhone, phone, message, "", "")
	if err != nil {
		fmt.Println("Ошибка отправки SMS:", err)
	} else {
		fmt.Println("SMS успешно отправлено на", phone)
	}
}
