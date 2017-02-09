// Copyright 2014 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

// Package ldap provide functions & structure to query a LDAP ldap directory
// For now, it's mainly tested again an MS Active Directory service, see README.md for more information
package ldap

import (
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"

	"gopkg.in/ldap.v2"

	"github.com/gogits/gogs/modules/log"
)

type SecurityProtocol int

// Basic LDAP authentication service
type Source struct {
	Name              string // canonical name (ie. corporate.ad)
	URL               string // LDAP host
	SkipVerify        bool
	StartTLS          bool
	BindDN            string // DN to bind with
	BindPassword      string // Bind DN password
	UserBase          string // Base search path for users
	UserDN            string // Template for the DN of the user for simple auth
	AttributeUsername string // Username attribute
	AttributeName     string // First name attribute
	AttributeSurname  string // Surname attribute
	AttributeMail     string // E-mail attribute
	AttributesInBind  bool   // fetch attributes in bind context (not user)
	Filter            string // Query filter to validate entry
	AdminFilter       string // Query filter to check if user is admin
	Enabled           bool   // if this source is disabled
}

func (ls *Source) sanitizedUserQuery(username string) (string, bool) {
	// See http://tools.ietf.org/search/rfc4515
	badCharacters := "\x00()*\\"
	if strings.ContainsAny(username, badCharacters) {
		log.Debug("'%s' contains invalid query characters. Aborting.", username)
		return "", false
	}

	return fmt.Sprintf(ls.Filter, username), true
}

func (ls *Source) sanitizedUserDN(username string) (string, bool) {
	// See http://tools.ietf.org/search/rfc4514: "special characters"
	badCharacters := "\x00()*\\,='\"#+;<>"
	if strings.ContainsAny(username, badCharacters) || strings.HasPrefix(username, " ") || strings.HasSuffix(username, " ") {
		log.Debug("'%s' contains invalid DN characters. Aborting.", username)
		return "", false
	}

	return fmt.Sprintf(ls.UserDN, username), true
}

func (ls *Source) findUserDN(l *ldap.Conn, name string) (string, bool) {
	log.Trace("Search for LDAP user: %s", name)
	if ls.BindDN != "" && ls.BindPassword != "" {
		err := l.Bind(ls.BindDN, ls.BindPassword)
		if err != nil {
			log.Debug("Failed to bind as BindDN[%s]: %v", ls.BindDN, err)
			return "", false
		}
		log.Trace("Bound as BindDN %s", ls.BindDN)
	} else {
		log.Trace("Proceeding with anonymous LDAP search.")
	}

	// A search for the user.
	userFilter, ok := ls.sanitizedUserQuery(name)
	if !ok {
		return "", false
	}

	log.Trace("Searching for DN using filter %s and base %s", userFilter, ls.UserBase)
	search := ldap.NewSearchRequest(
		ls.UserBase, ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 0,
		false, userFilter, []string{}, nil)

	// Ensure we found a user
	sr, err := l.Search(search)
	if err != nil || len(sr.Entries) < 1 {
		log.Debug("Failed search using filter[%s]: %v", userFilter, err)
		return "", false
	} else if len(sr.Entries) > 1 {
		log.Debug("Filter '%s' returned more than one user.", userFilter)
		return "", false
	}

	userDN := sr.Entries[0].DN
	if userDN == "" {
		log.Error(4, "LDAP search was successful, but found no DN!")
		return "", false
	}

	return userDN, true
}

func dial(ls *Source) (*ldap.Conn, error) {
	log.Trace("Dialing %s (skip cert verification: %v, start TLS: %v)", ls.URL, ls.SkipVerify, ls.StartTLS)

	//// URL Parsing
	ldapUrl := ls.URL
	ldapiHost := ""

	// Fix ldapi URLs (1/2): ~ by removing and saving the host part for later.
	if strings.HasPrefix(ldapUrl, "ldapi://") {
		x := strings.IndexAny(ldapUrl[8:], "/?#")
		if x >= 0 {
			ldapiHost = ldapUrl[8 : 8+x]
			ldapUrl = "ldapi://" + ldapUrl[8+x:]
		} else {
			ldapiHost = ldapUrl[8:]
			ldapUrl = "ldapi://"
		}
	}

	// Parse the URL
	u, err := url.Parse(ldapUrl)
	if err != nil {
		return nil, err
	}

	// Fix ldapi URLs (2/2): ~ by injecting the saved and decoded host part into the parsed URL struct.
	if ldapiHost != "" {
		u.Host, err = url.QueryUnescape(ldapiHost)
		if err != nil {
			return nil, fmt.Errorf("Unescape hostpart of ldapi URL: %v", err)
		}
	}

	if u.User != nil || u.Path != "" || u.Fragment != "" || u.RawQuery != "" || u.Opaque != "" {
		return nil, errors.New("LDAP URLs (for now) do not support pathes, fragments, querries or opaque form")
	}

	//// Dial
	// ldapI
	if u.Scheme == "ldapi" {
		conn, err := ldap.Dial("unix", u.Host)
		if err != nil {
			return nil, fmt.Errorf("Dial: %v", err)
		}

		return conn, nil
	}

	// Common stuff dor ldap / ldapS
	host, port, err := net.SplitHostPort(u.Host)
	if err != nil {
		return nil, fmt.Errorf("Failed splitting adress in host and port part: %v", err)
	}

	tlsCfg := &tls.Config{
		ServerName:         host,
		InsecureSkipVerify: ls.SkipVerify,
	}

	// ldapS
	if u.Scheme == "ldaps" {
		if port == "" {
			port = "636"
		}

		conn, err := ldap.DialTLS("tcp", fmt.Sprintf("%s:%d", host, port), tlsCfg)
		if err != nil {
			return nil, fmt.Errorf("DialTLS: %v", err)
		}

		return conn, nil
	}

	// ldap
	if u.Scheme == "ldap" {
		if port == "" {
			port = "389"
		}

		conn, err := ldap.Dial("tcp", fmt.Sprintf("%s:%d", host, port))
		if err != nil {
			return nil, fmt.Errorf("Dial: %v", err)
		}

		if ls.StartTLS == ls.StartTLS {
			if err = conn.StartTLS(tlsCfg); err != nil {
				conn.Close()
				return nil, fmt.Errorf("StartTLS: %v", err)
			}
		}

		return conn, nil
	}

	return nil, errors.New("The URL dos not has a valid LDAP scheme ('ldap://', 'ldaps://' or 'ldapi://'")
}

func bindUser(l *ldap.Conn, userDN, passwd string) error {
	log.Trace("Binding with userDN: %s", userDN)
	err := l.Bind(userDN, passwd)
	if err != nil {
		log.Debug("LDAP auth. failed for %s, reason: %v", userDN, err)
		return err
	}
	log.Trace("Bound successfully with userDN: %s", userDN)
	return err
}

// searchEntry : search an LDAP source if an entry (name, passwd) is valid and in the specific filter
func (ls *Source) SearchEntry(name, passwd string, directBind bool) (string, string, string, string, bool, bool) {
	// See https://tools.ietf.org/search/rfc4513#section-5.1.2
	if len(passwd) == 0 {
		log.Debug("Auth. failed for %s, password cannot be empty")
		return "", "", "", "", false, false
	}
	l, err := dial(ls)
	if err != nil {
		log.Error(4, "LDAP Connect error, %s:%v", ls.URL, err)
		ls.Enabled = false
		return "", "", "", "", false, false
	}
	defer l.Close()

	var userDN string
	if directBind {
		log.Trace("LDAP will bind directly via UserDN template: %s", ls.UserDN)

		var ok bool
		userDN, ok = ls.sanitizedUserDN(name)
		if !ok {
			return "", "", "", "", false, false
		}
	} else {
		log.Trace("LDAP will use BindDN.")

		var found bool
		userDN, found = ls.findUserDN(l, name)
		if !found {
			return "", "", "", "", false, false
		}
	}

	if directBind || !ls.AttributesInBind {
		// binds user (checking password) before looking-up attributes in user context
		err = bindUser(l, userDN, passwd)
		if err != nil {
			return "", "", "", "", false, false
		}
	}

	userFilter, ok := ls.sanitizedUserQuery(name)
	if !ok {
		return "", "", "", "", false, false
	}

	log.Trace("Fetching attributes '%v', '%v', '%v', '%v' with filter %s and base %s", ls.AttributeUsername, ls.AttributeName, ls.AttributeSurname, ls.AttributeMail, userFilter, userDN)
	search := ldap.NewSearchRequest(
		userDN, ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 0, false, userFilter,
		[]string{ls.AttributeUsername, ls.AttributeName, ls.AttributeSurname, ls.AttributeMail},
		nil)

	sr, err := l.Search(search)
	if err != nil {
		log.Error(4, "LDAP Search failed unexpectedly! (%v)", err)
		return "", "", "", "", false, false
	} else if len(sr.Entries) < 1 {
		if directBind {
			log.Error(4, "User filter inhibited user login.")
		} else {
			log.Error(4, "LDAP Search failed unexpectedly! (0 entries)")
		}

		return "", "", "", "", false, false
	}

	username := sr.Entries[0].GetAttributeValue(ls.AttributeUsername)
	firstname := sr.Entries[0].GetAttributeValue(ls.AttributeName)
	surname := sr.Entries[0].GetAttributeValue(ls.AttributeSurname)
	mail := sr.Entries[0].GetAttributeValue(ls.AttributeMail)

	isAdmin := false
	if len(ls.AdminFilter) > 0 {
		log.Trace("Checking admin with filter %s and base %s", ls.AdminFilter, userDN)
		search = ldap.NewSearchRequest(
			userDN, ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 0, false, ls.AdminFilter,
			[]string{ls.AttributeName},
			nil)

		sr, err = l.Search(search)
		if err != nil {
			log.Error(4, "LDAP Admin Search failed unexpectedly! (%v)", err)
		} else if len(sr.Entries) < 1 {
			log.Error(4, "LDAP Admin Search failed")
		} else {
			isAdmin = true
		}
	}

	if !directBind && ls.AttributesInBind {
		// binds user (checking password) after looking-up attributes in BindDN context
		err = bindUser(l, userDN, passwd)
		if err != nil {
			return "", "", "", "", false, false
		}
	}

	return username, firstname, surname, mail, isAdmin, true
}
