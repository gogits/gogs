// Copyright 2014 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package v1

import (
	"github.com/Unknwon/com"

	api "github.com/gogits/go-gogs-client"

	"github.com/gogits/gogs/models"
	"github.com/gogits/gogs/modules/base"
	"github.com/gogits/gogs/modules/middleware"
)

const (
	USERS     base.TplName = "admin/user/list"
	USER_NEW  base.TplName = "admin/user/new"
	USER_EDIT base.TplName = "admin/user/edit"
)

// ToApiUser converts user to API format.
func ToApiUser(u *models.User) *api.User {
	return &api.User{
		ID:        u.Id,
		UserName:  u.Name,
		FullName:  u.FullName,
		Email:     u.Email,
		AvatarUrl: u.AvatarLink(),
	}
}

func SearchUsers(ctx *middleware.Context) {
	opt := models.SearchOption{
		Keyword: ctx.Query("q"),
		Limit:   com.StrTo(ctx.Query("limit")).MustInt(),
	}
	if opt.Limit == 0 {
		opt.Limit = 10
	}

	us, err := models.SearchUserByName(opt)
	if err != nil {
		ctx.JSON(500, map[string]interface{}{
			"ok":    false,
			"error": err.Error(),
		})
		return
	}

	results := make([]*api.User, len(us))
	for i := range us {
		results[i] = &api.User{
			ID:        us[i].Id,
			UserName:  us[i].Name,
			AvatarUrl: us[i].AvatarLink(),
			FullName:  us[i].FullName,
		}
		if ctx.IsSigned {
			results[i].Email = us[i].Email
		}
	}

	ctx.Render.JSON(200, map[string]interface{}{
		"ok":   true,
		"data": results,
	})
}

// GET /users/:username
func GetUserInfo(ctx *middleware.Context) {
	u, err := models.GetUserByName(ctx.Params(":username"))
	if err != nil {
		if models.IsErrUserNotExist(err) {
			ctx.Error(404)
		} else {
			ctx.APIError(500, "GetUserByName", err)
		}
		return
	}

	// Hide user e-mail when API caller isn't signed in.
	if !ctx.IsSigned {
		u.Email = ""
	}
	ctx.JSON(200, &api.User{u.Id, u.Name, u.FullName, u.Email, u.AvatarLink()})
}

// POST /users
func CreateUser(ctx *middleware.Context, opt api.CreateUserOption) {
	if !ctx.User.IsAdmin {
		ctx.Error(403)
		return
	}

	u := &models.User{
		Name:      opt.Name,
		Email:     opt.Email,
		Passwd:    opt.Password,
		IsActive:  true,
		LoginType: models.PLAIN,
	}

	if err := models.CreateUser(u); err != nil {
		switch {
		case models.IsErrUserAlreadyExist(err):
			ctx.Data["Err_UserName"] = true
			ctx.RenderWithErr(ctx.Tr("form.username_been_taken"), USER_NEW, &opt)
		case models.IsErrEmailAlreadyUsed(err):
			ctx.Data["Err_Email"] = true
			ctx.RenderWithErr(ctx.Tr("form.email_been_used"), USER_NEW, &opt)
		case models.IsErrNameReserved(err):
			ctx.Data["Err_UserName"] = true
			ctx.RenderWithErr(ctx.Tr("user.form.name_reserved", err.(models.ErrNameReserved).Name), USER_NEW, &opt)
		case models.IsErrNamePatternNotAllowed(err):
			ctx.Data["Err_UserName"] = true
			ctx.RenderWithErr(ctx.Tr("user.form.name_pattern_not_allowed", err.(models.ErrNamePatternNotAllowed).Pattern), USER_NEW, &opt)
		default:
			ctx.Handle(500, "CreateUser", err)
		}
		return
	}

	ctx.JSON(201, &api.User{u.Id, u.Name, u.FullName, u.Email, u.AvatarLink()})
}
