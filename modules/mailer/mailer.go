// Copyright 2014 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package mailer

import (
	"crypto/tls"
	"fmt"
	"net"
	"os"
	"time"
	"strconv"
	"net/textproto"

	"gopkg.in/gomail.v2"

	"github.com/gogits/gogs/modules/log"
	"github.com/gogits/gogs/modules/setting"
)

type Message struct {
	Info string // Message information for log purpose.
	*gomail.Message
}

// NewMessageFrom creates new mail message object with custom From header.
func NewMessageFrom(to []string, from, subject, body string) *Message {
	msg := gomail.NewMessage()
	msg.SetHeader("From", from)
	msg.SetHeader("To", to...)
	msg.SetHeader("Subject", subject)
	msg.SetDateHeader("Date", time.Now())
	msg.SetBody("text/plain", body)
	msg.AddAlternative("text/html", body)

	return &Message{
		Message: msg,
	}
}

// NewMessage creates new mail message object with default From header.
func NewMessage(to []string, subject, body string) *Message {
	return NewMessageFrom(to, setting.MailService.From, subject, body)
}

func newDialer(opts *setting.Mailer) (*gomail.Dialer, error) {
	host, port, err := net.SplitHostPort(opts.Host)
	if err != nil {
		return nil, fmt.Errorf("Failed to convert hostname %s: %v", opts.Host, err)
	}

	portI, err := strconv.Atoi(port)
	if err != nil {
		return nil, fmt.Errorf("Failed to convert '%s' to a port number", port)
	}

	dialer := &gomail.Dialer {
		Host: host,
		Port: portI,
		Username: opts.User,
		Password: opts.Passwd,
		TLSConfig: &tls.Config {
			InsecureSkipVerify: opts.SkipVerify,
			ServerName:         host,
		},
	}

	if portI == 465 {
		dialer.SSL = true
	} else {
		dialer.SSL = false
	}

	hostname := opts.HeloHostname
	if len(hostname) == 0 {
		hostname, err = os.Hostname()
		if err != nil {
			return nil, err
		}
	}
	dialer.LocalName = hostname

	if opts.UseCertificate {
		cert, err := tls.LoadX509KeyPair(opts.CertFile, opts.KeyFile)
		if err != nil {
			return nil, err
		}
		dialer.TLSConfig.Certificates = []tls.Certificate{cert}
	}

	return dialer, nil;
}

func Test(opts *setting.Mailer) error {
	dialer, err := newDialer(opts)
	if err != nil {
		return err
	}

	log.Debug("Mailer: Dialing %s", opts.Host)
	conn, err := dialer.Dial()
	if err != nil {
		if terr, ok := err.(*textproto.Error); ok {
			terr.Msg = "Failed to connect: " + terr.Msg
		}
		return err
	}
	conn.Close()
	return nil
}

func Send(msg *Message) error {
	opts := setting.MailService

	dialer, err := newDialer(opts)
	if err != nil {
		return err
	}

	log.Debug("Mailer: Dialing %s", opts.Host)
	conn, err := dialer.Dial()
	if err != nil {
		if terr, ok := err.(*textproto.Error); ok {
			terr.Msg = "Failed to connect: " + terr.Msg
		}
		return err
	}
	defer conn.Close()

	if err := conn.Send(opts.From, msg.GetHeader("To"), msg.Message); err != nil {
		return err
	}
	log.Trace("E-mail sent %s: %s", msg.GetHeader("To"), msg.Info)
	return nil
}

func processMailQueue() {
	opts := setting.MailService

	dialer, err := newDialer(opts)
	if err != nil {
		return
	}
	open := false
	var conn gomail.SendCloser

	for {
		select {
		// A new message is pending
		case msg := <-mailQueue:
			// Dial SMTP if neccessary
			if !open {
				var err error
				if conn, err = dialer.Dial(); err != nil {
					log.Error(4, "Fail to connect to SMTP server: %v", err)
				}
				open = true
			}

			// Send SMTP message
			log.Trace("New e-mail sending request %s: %s", msg.GetHeader("To"), msg.Info)
			if err := conn.Send(opts.From, msg.GetHeader("To"), msg.Message); err != nil {
				log.Error(4, "Fail to send e-mails %s: %s - %v", msg.GetHeader("To"), msg.Info, err)
			} else {
				log.Trace("E-mails sent %s: %s", msg.GetHeader("To"), msg.Info)
			}

		// Close the connection to the SMTP server if no email was sent in the last 30 seconds
		case <-time.After(30 * time.Second):
			if open {
				if err := conn.Close(); err != nil {
					log.Error(4, "Fail to close SMTP connection: %v", err)
				}
				open = false
			}
		}
	}
}

var mailQueue chan *Message

func NewContext() {
	// Need to check if mailQueue is nil because in during reinstall (user had installed
	// before but swithed install lock off), this function will be called again
	// while mail queue is already processing tasks, and produces a race condition.
	if setting.MailService == nil || mailQueue != nil {
		return
	}

	mailQueue = make(chan *Message, setting.MailService.QueueLength)
	go processMailQueue()
}

func SendAsync(msg *Message) {
	go func() {
		mailQueue <- msg
	}()
}

