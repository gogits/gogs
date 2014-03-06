// Copyright 2014 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package routers

import (
	"github.com/gogits/gogs/modules/base"
	"github.com/gogits/gogs/routers/user"
	"github.com/martini-contrib/render"
	"github.com/martini-contrib/sessions"
)

func Home(r render.Render, data base.TmplData, session sessions.Session) {
	if user.IsSignedIn(session) {
		user.Dashboard(r, data, session)
		return
	}
	data["PageIsHome"] = true
	r.HTML(200, "home", data)
}
