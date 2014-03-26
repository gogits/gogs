// Copyright 2014 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"path"
	"path/filepath"
	"strings"

	"github.com/codegangsta/martini"

	"github.com/gogits/webdav"

	"github.com/gogits/gogs/models"
	"github.com/gogits/gogs/modules/auth"
	"github.com/gogits/gogs/modules/base"
	"github.com/gogits/gogs/modules/log"
	"github.com/gogits/gogs/modules/middleware"
)

func Create(ctx *middleware.Context, form auth.CreateRepoForm) {
	ctx.Data["Title"] = "Create repository"
	ctx.Data["PageIsNewRepo"] = true // For navbar arrow.
	ctx.Data["LanguageIgns"] = models.LanguageIgns
	ctx.Data["Licenses"] = models.Licenses

	if ctx.Req.Method == "GET" {
		ctx.HTML(200, "repo/create")
		return
	}

	if ctx.HasError() {
		ctx.HTML(200, "repo/create")
		return
	}

	_, err := models.CreateRepository(ctx.User, form.RepoName, form.Description,
		form.Language, form.License, form.Visibility == "private", form.InitReadme == "on")
	if err == nil {
		log.Trace("%s Repository created: %s/%s", ctx.Req.RequestURI, ctx.User.LowerName, form.RepoName)
		ctx.Redirect("/" + ctx.User.Name + "/" + form.RepoName)
		return
	} else if err == models.ErrRepoAlreadyExist {
		ctx.RenderWithErr("Repository name has already been used", "repo/create", &form)
		return
	} else if err == models.ErrRepoNameIllegal {
		ctx.RenderWithErr(models.ErrRepoNameIllegal.Error(), "repo/create", &form)
		return
	}
	ctx.Handle(200, "repo.Create", err)
}

func Single(ctx *middleware.Context, params martini.Params) {
	if !ctx.Repo.IsValid {
		return
	}

	// Get tree path
	treename := params["_1"]

	if len(treename) > 0 && treename[len(treename)-1] == '/' {
		ctx.Redirect("/" + ctx.Repo.Owner.LowerName + "/" +
			ctx.Repo.Repository.Name + "/src/" + params["branchname"] + "/" + treename[:len(treename)-1])
		return
	}

	ctx.Data["IsRepoToolbarSource"] = true

	// Branches.
	brs, err := models.GetBranches(params["username"], params["reponame"])
	if err != nil {
		//log.Error("repo.Single(GetBranches): %v", err)
		ctx.Handle(404, "repo.Single(GetBranches)", err)
		return
	} else if ctx.Repo.Repository.IsBare {
		ctx.Data["IsBareRepo"] = true
		ctx.HTML(200, "repo/single")
		return
	}

	ctx.Data["Branches"] = brs

	repoFile, err := models.GetTargetFile(params["username"], params["reponame"],
		params["branchname"], params["commitid"], treename)

	if err != nil && err != models.ErrRepoFileNotExist {
		//log.Error("repo.Single(GetTargetFile): %v", err)
		ctx.Handle(404, "repo.Single(GetTargetFile)", err)
		return
	}

	branchLink := "/" + ctx.Repo.Owner.LowerName + "/" + ctx.Repo.Repository.Name + "/src/" + params["branchname"]
	rawLink := "/" + ctx.Repo.Owner.LowerName + "/" + ctx.Repo.Repository.Name + "/raw/" + params["branchname"]

	if len(treename) != 0 && repoFile == nil {
		ctx.Handle(404, "repo.Single", nil)
		return
	}

	if repoFile != nil && repoFile.IsFile() {
		if blob, err := repoFile.LookupBlob(); err != nil {
			ctx.Handle(404, "repo.Single(repoFile.LookupBlob)", err)
		} else {
			ctx.Data["FileSize"] = repoFile.Size
			ctx.Data["IsFile"] = true
			ctx.Data["FileName"] = repoFile.Name
			ext := path.Ext(repoFile.Name)
			if len(ext) > 0 {
				ext = ext[1:]
			}
			ctx.Data["FileExt"] = ext
			ctx.Data["FileLink"] = rawLink + "/" + treename

			data := blob.Contents()
			_, isTextFile := base.IsTextFile(data)
			ctx.Data["FileIsText"] = isTextFile

			readmeExist := base.IsMarkdownFile(repoFile.Name) || base.IsReadmeFile(repoFile.Name)
			ctx.Data["ReadmeExist"] = readmeExist
			if readmeExist {
				ctx.Data["FileContent"] = string(base.RenderMarkdown(data, ""))
			} else {
				if isTextFile {
					ctx.Data["FileContent"] = string(data)
				}
			}
		}

	} else {
		// Directory and file list.
		files, err := models.GetReposFiles(params["username"], params["reponame"],
			params["branchname"], params["commitid"], treename)
		if err != nil {
			//log.Error("repo.Single(GetReposFiles): %v", err)
			ctx.Handle(404, "repo.Single(GetReposFiles)", err)
			return
		}

		ctx.Data["Files"] = files

		var readmeFile *models.RepoFile

		for _, f := range files {
			if !f.IsFile() || !base.IsReadmeFile(f.Name) {
				continue
			} else {
				readmeFile = f
				break
			}
		}

		if readmeFile != nil {
			ctx.Data["ReadmeInSingle"] = true
			ctx.Data["ReadmeExist"] = true
			if blob, err := readmeFile.LookupBlob(); err != nil {
				ctx.Handle(404, "repo.Single(readmeFile.LookupBlob)", err)
				return
			} else {
				ctx.Data["FileSize"] = readmeFile.Size
				ctx.Data["FileLink"] = rawLink + "/" + treename
				data := blob.Contents()
				_, isTextFile := base.IsTextFile(data)
				ctx.Data["FileIsText"] = isTextFile
				ctx.Data["FileName"] = readmeFile.Name
				if isTextFile {
					ctx.Data["FileContent"] = string(base.RenderMarkdown(data, branchLink))
				}
			}
		}
	}

	ctx.Data["Username"] = params["username"]
	ctx.Data["Reponame"] = params["reponame"]

	var treenames []string
	Paths := make([]string, 0)

	if len(treename) > 0 {
		treenames = strings.Split(treename, "/")
		for i, _ := range treenames {
			Paths = append(Paths, strings.Join(treenames[0:i+1], "/"))
		}

		ctx.Data["HasParentPath"] = true
		if len(Paths)-2 >= 0 {
			ctx.Data["ParentPath"] = "/" + Paths[len(Paths)-2]
		}
	}

	// Get latest commit according username and repo name
	commit, err := models.GetCommit(params["username"], params["reponame"],
		params["branchname"], params["commitid"])
	if err != nil {
		log.Error("repo.Single(GetCommit): %v", err)
		ctx.Handle(404, "repo.Single(GetCommit)", err)
		return
	}
	ctx.Data["LastCommit"] = commit

	ctx.Data["Paths"] = Paths
	ctx.Data["Treenames"] = treenames
	ctx.Data["BranchLink"] = branchLink
	ctx.HTML(200, "repo/single")
}

func SingleDownload(ctx *middleware.Context, params martini.Params) {
	if !ctx.Repo.IsValid {
		ctx.Handle(404, "repo.SingleDownload", nil)
		return
	}

	// Get tree path
	treename := params["_1"]

	repoFile, err := models.GetTargetFile(params["username"], params["reponame"],
		params["branchname"], params["commitid"], treename)

	if err != nil {
		ctx.Handle(404, "repo.SingleDownload(GetTargetFile)", err)
		return
	}

	blob, err := repoFile.LookupBlob()
	if err != nil {
		ctx.Handle(404, "repo.SingleDownload(LookupBlob)", err)
		return
	}

	data := blob.Contents()
	contentType, isTextFile := base.IsTextFile(data)
	ctx.Res.Header().Set("Content-Type", contentType)
	if !isTextFile {
		ctx.Res.Header().Set("Content-Type", contentType)
		ctx.Res.Header().Set("Content-Disposition", "attachment; filename="+filepath.Base(treename))
		ctx.Res.Header().Set("Content-Transfer-Encoding", "binary")
	}
	ctx.Res.Write(data)
}

func Http(ctx *middleware.Context, params martini.Params) {
	/*if !ctx.Repo.IsValid {
		return
	}*/

	// TODO: access check

	username := params["username"]
	reponame := params["reponame"]
	if strings.HasSuffix(reponame, ".git") {
		reponame = reponame[:len(reponame)-4]
	}

	prefix := path.Join("/", username, params["reponame"])
	server := &webdav.Server{
		Fs:         webdav.Dir(models.RepoPath(username, reponame)),
		TrimPrefix: prefix,
		Listings:   true,
	}

	server.ServeHTTP(ctx.ResponseWriter, ctx.Req)
}

func Setting(ctx *middleware.Context, params martini.Params) {
	if !ctx.Repo.IsOwner {
		ctx.Handle(404, "repo.Setting", nil)
		return
	}

	ctx.Data["IsRepoToolbarSetting"] = true

	if ctx.Repo.Repository.IsBare {
		ctx.Data["IsBareRepo"] = true
		ctx.HTML(200, "repo/setting")
		return
	}

	var title string
	if t, ok := ctx.Data["Title"].(string); ok {
		title = t
	}

	ctx.Data["Title"] = title + " - settings"
	ctx.HTML(200, "repo/setting")
}

func SettingPost(ctx *middleware.Context) {
	if !ctx.Repo.IsOwner {
		ctx.Error(404)
		return
	}

	switch ctx.Query("action") {
	case "update":
		ctx.Repo.Repository.Description = ctx.Query("desc")
		ctx.Repo.Repository.Website = ctx.Query("site")
		if err := models.UpdateRepository(ctx.Repo.Repository); err != nil {
			ctx.Handle(404, "repo.SettingPost(update)", err)
			return
		}
		ctx.Data["IsSuccess"] = true
		ctx.HTML(200, "repo/setting")
		log.Trace("%s Repository updated: %s/%s", ctx.Req.RequestURI, ctx.User.LowerName, ctx.Repo.Repository.LowerName)
	case "delete":
		if len(ctx.Repo.Repository.Name) == 0 || ctx.Repo.Repository.Name != ctx.Query("repository") {
			ctx.Data["ErrorMsg"] = "Please make sure you entered repository name is correct."
			ctx.HTML(200, "repo/setting")
			return
		}

		if err := models.DeleteRepository(ctx.User.Id, ctx.Repo.Repository.Id, ctx.User.LowerName); err != nil {
			ctx.Handle(200, "repo.Delete", err)
			return
		}

		log.Trace("%s Repository deleted: %s/%s", ctx.Req.RequestURI, ctx.User.LowerName, ctx.Repo.Repository.LowerName)
		ctx.Redirect("/")
	}
}

func Action(ctx *middleware.Context, params martini.Params) {
	var err error
	switch params["action"] {
	case "watch":
		err = models.WatchRepo(ctx.User.Id, ctx.Repo.Repository.Id, true)
	case "unwatch":
		err = models.WatchRepo(ctx.User.Id, ctx.Repo.Repository.Id, false)
	case "desc":
		if !ctx.Repo.IsOwner {
			ctx.Error(404)
			return
		}

		ctx.Repo.Repository.Description = ctx.Query("desc")
		ctx.Repo.Repository.Website = ctx.Query("site")
		err = models.UpdateRepository(ctx.Repo.Repository)
	}

	if err != nil {
		log.Error("repo.Action(%s): %v", params["action"], err)
		ctx.JSON(200, map[string]interface{}{
			"ok":  false,
			"err": err.Error(),
		})
		return
	}
	ctx.JSON(200, map[string]interface{}{
		"ok": true,
	})
}
