// Copyright 2014 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

const (
	SLACK_COLOR string = "#dd4b39"
)

type Slack struct {
	Domain  string `json:"domain"`
	Token   string `json:"token"`
	Channel string `json:"channel"`
}

type SlackPayload struct {
	Channel     string            `json:"channel"`
	Text        string            `json:"text"`
	Username    string            `json:"username"`
	IconUrl     string            `json:"icon_url"`
	UnfurlLinks int               `json:"unfurl_links"`
	LinkNames   int               `json:"link_names"`
	Attachments []SlackAttachment `json:"attachments"`
}

type SlackAttachment struct {
	Color string `json:"color"`
	Text  string `json:"text"`
}

func GetSlackURL(domain string, token string) string {
	return fmt.Sprintf(
		"https://%s.slack.com/services/hooks/incoming-webhook?token=%s",
		domain,
		token,
	)
}

func (p SlackPayload) GetJSONPayload() ([]byte, error) {
	data, err := json.Marshal(p)
	if err != nil {
		return []byte{}, err
	}
	return data, nil
}

func GetSlackPayload(p *Payload, meta string) (*SlackPayload, error) {
	slack := &Slack{}
	slackPayload := &SlackPayload{}
	if err := json.Unmarshal([]byte(meta), &slack); err != nil {
		return slackPayload, errors.New("GetSlackPayload meta json:" + err.Error())
	}

	// TODO: handle different payload types: push, new branch, delete branch etc.
	// when they are added to gogs. Only handles push now
	return getSlackPushPayload(p, slack)
}

func getSlackPushPayload(p *Payload, slack *Slack) (*SlackPayload, error) {
	// n new commits
	refSplit := strings.Split(p.Ref, "/")
	branchName := refSplit[len(refSplit)-1]
	var commitString string

	// TODO: add commit compare before/after link when gogs adds it
	if len(p.Commits) == 1 {
		commitString = "1 new commit"
	} else {
		commitString = fmt.Sprintf("%d new commits", len(p.Commits))
	}

	text := fmt.Sprintf("[%s:%s] %s pushed by %s", p.Repo.Name, branchName, commitString, p.Pusher.Name)
	var attachmentText string

	// for each commit, generate attachment text
	for i, commit := range p.Commits {
		attachmentText += fmt.Sprintf("<%s|%s>: %s - %s", commit.Url, commit.Id[:7], SlackFormatter(commit.Message), commit.Author.Name)
		// add linebreak to each commit but the last
		if i < len(p.Commits)-1 {
			attachmentText += "\n"
		}
	}

	slackAttachments := []SlackAttachment{{Color: SLACK_COLOR, Text: attachmentText}}

	return &SlackPayload{
		Channel:     slack.Channel,
		Text:        text,
		Username:    "gogs",
		IconUrl:     "https://raw.githubusercontent.com/gogits/gogs/master/public/img/favicon.png",
		UnfurlLinks: 0,
		LinkNames:   0,
		Attachments: slackAttachments,
	}, nil
}

// see: https://api.slack.com/docs/formatting
func SlackFormatter(s string) string {
	// take only first line of commit
	first := strings.Split(s, "\n")[0]
	// replace & < >
	first = strings.Replace(first, "&", "&amp;", -1)
	first = strings.Replace(first, "<", "&lt;", -1)
	first = strings.Replace(first, ">", "&gt;", -1)
	return first
}
