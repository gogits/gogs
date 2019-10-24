// Copyright 2014 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package admin

import (
	"gogs.io/gogs/internal/context"
	route2 "gogs.io/gogs/internal/route"
	"gogs.io/gogs/internal/setting"
	"gogs.io/gogs/db"
)

const (
	ORGS = "admin/org/list"
)

func Organizations(c *context.Context) {
	c.Data["Title"] = c.Tr("admin.organizations")
	c.Data["PageIsAdmin"] = true
	c.Data["PageIsAdminOrganizations"] = true

	route2.RenderUserSearch(c, &route2.UserSearchOptions{
		Type:     db.USER_TYPE_ORGANIZATION,
		Counter:  db.CountOrganizations,
		Ranger:   db.Organizations,
		PageSize: setting.UI.Admin.OrgPagingNum,
		OrderBy:  "id ASC",
		TplName:  ORGS,
	})
}
