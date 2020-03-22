// Copyright 2014 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package org

import (
	"strings"

	log "unknwon.dev/clog/v2"

	"gogs.io/gogs/internal/conf"
	"gogs.io/gogs/internal/context"
	"gogs.io/gogs/internal/db"
	"gogs.io/gogs/internal/form"
	"gogs.io/gogs/internal/route/user"
)

const (
	SETTINGS_OPTIONS        = "org/settings/options"
	SETTINGS_DELETE         = "org/settings/delete"
	tmplOrgSettingsWebhooks = "org/settings/webhooks"
)

func Settings(c *context.Context) {
	c.Title("org.settings")
	c.Data["PageIsSettingsOptions"] = true
	c.Success(SETTINGS_OPTIONS)
}

func SettingsPost(c *context.Context, f form.UpdateOrgSetting) {
	c.Title("org.settings")
	c.Data["PageIsSettingsOptions"] = true

	if c.HasError() {
		c.Success(SETTINGS_OPTIONS)
		return
	}

	org := c.Org.Organization

	// Check if organization name has been changed.
	if org.LowerName != strings.ToLower(f.Name) {
		isExist, err := db.IsUserExist(org.ID, f.Name)
		if err != nil {
			c.Error(err, "check if user exists")
			return
		} else if isExist {
			c.Data["OrgName"] = true
			c.RenderWithErr(c.Tr("form.username_been_taken"), SETTINGS_OPTIONS, &f)
			return
		} else if err = db.ChangeUserName(org, f.Name); err != nil {
			c.Data["OrgName"] = true
			switch {
			case db.IsErrNameReserved(err):
				c.RenderWithErr(c.Tr("user.form.name_reserved"), SETTINGS_OPTIONS, &f)
			case db.IsErrNamePatternNotAllowed(err):
				c.RenderWithErr(c.Tr("user.form.name_pattern_not_allowed"), SETTINGS_OPTIONS, &f)
			default:
				c.Error(err, "change user name")
			}
			return
		}
		// reset c.org.OrgLink with new name
		c.Org.OrgLink = conf.Server.Subpath + "/org/" + f.Name
		log.Trace("Organization name changed: %s -> %s", org.Name, f.Name)
	}
	// In case it's just a case change.
	org.Name = f.Name
	org.LowerName = strings.ToLower(f.Name)

	if c.User.IsAdmin {
		org.MaxRepoCreation = f.MaxRepoCreation
	}

	org.FullName = f.FullName
	org.Description = f.Description
	org.Website = f.Website
	org.Location = f.Location
	if err := db.UpdateUser(org); err != nil {
		c.Error(err, "update user")
		return
	}
	log.Trace("Organization setting updated: %s", org.Name)
	c.Flash.Success(c.Tr("org.settings.update_setting_success"))
	c.Redirect(c.Org.OrgLink + "/settings")
}

func SettingsAvatar(c *context.Context, f form.Avatar) {
	f.Source = form.AVATAR_LOCAL
	if err := user.UpdateAvatarSetting(c, f, c.Org.Organization); err != nil {
		c.Flash.Error(err.Error())
	} else {
		c.Flash.Success(c.Tr("org.settings.update_avatar_success"))
	}

	c.Redirect(c.Org.OrgLink + "/settings")
}

func SettingsDeleteAvatar(c *context.Context) {
	if err := c.Org.Organization.DeleteAvatar(); err != nil {
		c.Flash.Error(err.Error())
	}

	c.Redirect(c.Org.OrgLink + "/settings")
}

func SettingsDelete(c *context.Context) {
	c.Title("org.settings")
	c.PageIs("SettingsDelete")

	org := c.Org.Organization
	if c.Req.Method == "POST" {
		if _, err := db.UserLogin(c.User.Name, c.Query("password"), c.User.LoginSource); err != nil {
			if db.IsErrUserNotExist(err) {
				c.RenderWithErr(c.Tr("form.enterred_invalid_password"), SETTINGS_DELETE, nil)
			} else {
				c.Error(err, "authenticate user")
			}
			return
		}

		if err := db.DeleteOrganization(org); err != nil {
			if db.IsErrUserOwnRepos(err) {
				c.Flash.Error(c.Tr("form.org_still_own_repo"))
				c.Redirect(c.Org.OrgLink + "/settings/delete")
			} else {
				c.Error(err, "delete organization")
			}
		} else {
			log.Trace("Organization deleted: %s", org.Name)
			c.Redirect(conf.Server.Subpath + "/")
		}
		return
	}

	c.Success(SETTINGS_DELETE)
}

func Webhooks(c *context.Context) {
	c.Title("org.settings")
	c.PageIs("SettingsHooks")
	c.Data["Description"] = c.Tr("org.settings.hooks_desc")
	c.Data["Types"] = conf.Webhook.Types

	ws, err := db.GetWebhooksByOrgID(c.Org.Organization.ID)
	if err != nil {
		c.Error(err, "get webhooks by organization ID")
		return
	}
	c.Data["Webhooks"] = ws

	c.Success(tmplOrgSettingsWebhooks)
}

func DeleteWebhook(c *context.Context) {
	if err := db.DeleteWebhookOfOrgByID(c.Org.Organization.ID, c.QueryInt64("id")); err != nil {
		c.Flash.Error("DeleteWebhookByOrgID: " + err.Error())
	} else {
		c.Flash.Success(c.Tr("repo.settings.webhook_deletion_success"))
	}

	c.JSONSuccess(map[string]interface{}{
		"redirect": c.Org.OrgLink + "/settings/hooks",
	})
}
