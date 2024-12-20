package main

import (
	"keypub/internal/mail"
	"log"
)

const (
	resend_key_path           = "/home/ubuntu/.keys/.resend"
	confirmation_mail_address = "confirmations@keypub.sh"
	from_name                 = "keypub.sh"
)

func main() {
	mail_sender, err := mail.NewMailSender(resend_key_path, confirmation_mail_address, from_name)
	if err != nil {
		log.Fatalf("cannot initialize mail sender: %s", err)
	}
	err = mail_sender.SendConfirmation("skariel@gmail.com", "67823", "dK4XQnV")
	if err != nil {
		log.Printf("error sending mail: %s", err)
	}
}
