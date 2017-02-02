// Copyright 2016 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"fmt"

	"github.com/Unknwon/com"

	"github.com/gogits/gogs/modules/log"
	"github.com/gogits/gogs/modules/mailer"
	"github.com/gogits/gogs/modules/markdown"
	"github.com/gogits/gogs/modules/setting"
)

func (issue *Issue) MailSubject() string {
	return fmt.Sprintf("[%s] %s (#%d)", issue.Repo.Name, issue.Title, issue.Index)
}

// mailerUser is a wrapper for satisfying mailer.User interface.
type mailerUser struct {
	user *User
}

func (this mailerUser) ID() int64 {
	return this.user.ID
}

func (this mailerUser) DisplayName() string {
	return this.user.DisplayName()
}

func (this mailerUser) Email() string {
	return this.user.Email
}

func (this mailerUser) GenerateActivateCode() string {
	return this.user.GenerateActivateCode()
}

func (this mailerUser) GenerateEmailActivateCode(email string) string {
	return this.user.GenerateEmailActivateCode(email)
}

func NewMailerUser(u *User) mailer.User {
	return mailerUser{u}
}

// mailerRepo is a wrapper for satisfying mailer.Repository interface.
type mailerRepo struct {
	repo *Repository
}

func (this mailerRepo) FullName() string {
	return this.repo.FullName()
}

func (this mailerRepo) HTMLURL() string {
	return this.repo.HTMLURL()
}

func (this mailerRepo) ComposeMetas() map[string]string {
	return this.repo.ComposeMetas()
}

func NewMailerRepo(repo *Repository) mailer.Repository {
	return mailerRepo{repo}
}

// mailerIssue is a wrapper for satisfying mailer.Issue interface.
type mailerIssue struct {
	issue *Issue
}

func (this mailerIssue) MailSubject() string {
	return this.issue.MailSubject()
}

func (this mailerIssue) Content() string {
	return this.issue.Content
}

func (this mailerIssue) HTMLURL() string {
	return this.issue.HTMLURL()
}

func NewMailerIssue(issue *Issue) mailer.Issue {
	return mailerIssue{issue}
}

// mailIssueCommentToParticipants can be used for both new issue creation and comment.
func mailIssueCommentToParticipants(issue *Issue, doer *User, mentions []string) error {
	if !setting.Service.EnableNotifyMail {
		return nil
	}

	// Mail wahtcers.
	watchers, err := GetWatchers(issue.RepoID)
	if err != nil {
		return fmt.Errorf("GetWatchers [%d]: %v", issue.RepoID, err)
	}

	tos := make([]string, 0, len(watchers)) // List of email addresses.
	names := make([]string, 0, len(watchers))
	for i := range watchers {
		if watchers[i].UserID == doer.ID {
			continue
		}

		to, err := GetUserByID(watchers[i].UserID)
		if err != nil {
			return fmt.Errorf("GetUserByID [%d]: %v", watchers[i].UserID, err)
		}
		if to.IsOrganization() {
			continue
		}

		tos = append(tos, to.Email)
		names = append(names, to.Name)
	}
	mailer.SendIssueCommentMail(NewMailerIssue(issue), NewMailerRepo(issue.Repo), NewMailerUser(doer), tos)

	// Mail mentioned people and exclude watchers.
	names = append(names, doer.Name)
	tos = make([]string, 0, len(mentions)) // list of user names.
	for i := range mentions {
		if com.IsSliceContainsStr(names, mentions[i]) {
			continue
		}

		tos = append(tos, mentions[i])
	}
	mailer.SendIssueMentionMail(NewMailerIssue(issue), NewMailerRepo(issue.Repo), NewMailerUser(doer), GetUserEmailsByNames(tos))

	return nil
}

// MailParticipants sends new issue thread created emails to repository watchers
// and mentioned people.
func (issue *Issue) MailParticipants() (err error) {
	mentions := markdown.FindAllMentions(issue.Content)
	if err = updateIssueMentions(x, issue.ID, mentions); err != nil {
		return fmt.Errorf("UpdateIssueMentions [%d]: %v", issue.ID, err)
	}

	if err = mailIssueCommentToParticipants(issue, issue.Poster, mentions); err != nil {
		log.Error(4, "mailIssueCommentToParticipants: %v", err)
	}

	return nil
}
