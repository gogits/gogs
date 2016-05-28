// Copyright 2016 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"strings"

	"github.com/gogits/gogs/modules/base"
	"github.com/gogits/gogs/modules/context"
	"github.com/gogits/gogs/modules/log"
	"github.com/gogits/gogs/modules/auth"
	"path"
	"github.com/gogits/gogs/modules/setting"
	"fmt"
	"net/http"
	"github.com/gogits/gogs/models"
)

const (
	UPLOAD base.TplName = "repo/upload"
)

func renderUploadSettings(ctx *context.Context) {
	ctx.Data["RequireDropzone"] = true
	ctx.Data["IsUploadEnabled"] = setting.UploadEnabled
	ctx.Data["UploadAllowedTypes"] = setting.UploadAllowedTypes
	ctx.Data["UploadMaxSize"] = setting.UploadMaxSize
	ctx.Data["UploadMaxFiles"] = setting.UploadMaxFiles
}

func UploadFile(ctx *context.Context) {
	ctx.Data["PageIsUpload"] = true

	userName := ctx.Repo.Owner.Name
	repoName := ctx.Repo.Repository.Name
	branchName := ctx.Repo.BranchName
	branchLink := ctx.Repo.RepoLink + "/src/" + branchName
	treeName := ctx.Repo.TreeName

	if ! ctx.Repo.IsWriter() {
		ctx.Redirect(EscapeUrl(ctx.Repo.RepoLink + "/src/" + branchName + "/" + treeName))
		return
	}

	treeNames := []string{""}
	if len(treeName) > 0 {
		treeNames = strings.Split(treeName, "/")
	}

	ctx.Data["UserName"] = userName
	ctx.Data["RepoName"] = repoName
	ctx.Data["BranchName"] = branchName
	ctx.Data["TreeName"] = treeName
	ctx.Data["TreeNames"] = treeNames
	ctx.Data["BranchLink"] = branchLink
	ctx.Data["CommitSummary"] = ""
	ctx.Data["CommitMessage"] = ""
	ctx.Data["CommitChoice"] = "direct"
	ctx.Data["NewBranchName"] = ""
	ctx.Data["CommitDirectlyToThisBranch"] = ctx.Tr("repo.commit_directly_to_this_branch", "<strong class=\"branch-name\">"+branchName+"</strong>")
	ctx.Data["CreateNewBranch"] = ctx.Tr("repo.create_new_branch", "<strong>"+ctx.Tr("repo.new_branch")+"</strong>")
	renderUploadSettings(ctx)

	ctx.HTML(200, UPLOAD)
}

func UploadFilePost(ctx *context.Context, form auth.UploadRepoFileForm) {
	ctx.Data["PageIsUpload"] = true
	renderUploadSettings(ctx)

	userName := ctx.Repo.Owner.Name
	repoName := ctx.Repo.Repository.Name
	oldBranchName := ctx.Repo.BranchName
	branchName := oldBranchName
	branchLink := ctx.Repo.RepoLink + "/src/" + branchName
	commitChoice := form.CommitChoice
	files := form.Files

	if commitChoice == "commit-to-new-branch" {
		branchName = form.NewBranchName
	}

	treeName := form.TreeName
	treeName = strings.Trim(treeName, " ")
	treeName = strings.Trim(treeName, "/")

	if ! ctx.Repo.IsWriter()  {
		ctx.Redirect(EscapeUrl(ctx.Repo.RepoLink + "/src/" + branchName + "/" + treeName))
		return
	}

	treeNames := []string{""}
	if len(treeName) > 0 {
		treeNames = strings.Split(treeName, "/")
	}

	ctx.Data["UserName"] = userName
	ctx.Data["RepoName"] = repoName
	ctx.Data["BranchName"] = branchName
	ctx.Data["TreeName"] = treeName
	ctx.Data["TreeNames"] = treeNames
	ctx.Data["BranchLink"] = branchLink
	ctx.Data["CommitSummary"] = form.CommitSummary
	ctx.Data["CommitMessage"] = form.CommitMessage
	ctx.Data["CommitChoice"] = commitChoice
	ctx.Data["NewBranchName"] = branchName
	ctx.Data["CommitDirectlyToThisBranch"] = ctx.Tr("repo.commit_directly_to_this_branch", "<strong class=\"branch-name\">"+oldBranchName+"</strong>")
	ctx.Data["CreateNewBranch"] = ctx.Tr("repo.create_new_branch", "<strong>"+ctx.Tr("repo.new_branch")+"</strong>")

	if ctx.HasError() {
		ctx.HTML(200, UPLOAD)
		return
	}

	if( oldBranchName != branchName ){
		if _, err := ctx.Repo.Repository.GetBranch(branchName); err == nil {
			ctx.Data["Err_Branchname"] = true
			ctx.RenderWithErr(ctx.Tr("repo.branch_already_exists"), UPLOAD, &form)
			log.Error(4, "%s: %s - %s", "BranchName", branchName, "Branch already exists")
			return
		}

	}

	treepath := ""
	for _, part := range treeNames {
		treepath = path.Join(treepath, part)
		entry, err := ctx.Repo.Commit.GetTreeEntryByPath(treepath)
		if err != nil {
			// Means there is no item with that name, so we're good
			break
		}
		if ! entry.IsDir() {
			ctx.Data["Err_Filename"] = true
			ctx.RenderWithErr(ctx.Tr("repo.directory_is_a_file"), UPLOAD, &form)
			log.Error(4, "%s: %s - %s", "UploadFile", treeName, "Directory given is a file")
			return
		}
	}

	message := ""
	if form.CommitSummary!="" {
		message = strings.Trim(form.CommitSummary, " ")
	} else {
		message = ctx.Tr("repo.add_files_to_dir", "'" + treeName + "'")
	}
	if strings.Trim(form.CommitMessage, " ")!="" {
		message += "\n\n" + strings.Trim(form.CommitMessage, " ")
	}

	if err := ctx.Repo.Repository.UploadRepoFiles(ctx.User, oldBranchName, branchName, treeName,  message, files); err != nil {
		ctx.Data["Err_Directory"] = true
		ctx.RenderWithErr(ctx.Tr("repo.unable_to_upload_files"), UPLOAD, &form)
		log.Error(4, "%s: %v", "UploadFile", err)
		return
	}

	// Leaving this off until forked repos that get a branch can compare with forks master and not upstream
	//if oldBranchName != branchName {
	//	ctx.Redirect(EscapeUrl(ctx.Repo.RepoLink + "/compare/" + oldBranchName + "..." + branchName))
	//} else {
		ctx.Redirect(EscapeUrl(ctx.Repo.RepoLink + "/src/" + branchName + "/" + treeName))
	//}
}

func UploadFileToServer(ctx *context.Context) {
	if !setting.UploadEnabled {
		ctx.Error(404, "upload is not enabled")
		return
	}

	file, header, err := ctx.Req.FormFile("file")
	if err != nil {
		ctx.Error(500, fmt.Sprintf("FormFile: %v", err))
		return
	}
	defer file.Close()

	buf := make([]byte, 1024)
	n, _ := file.Read(buf)
	if n > 0 {
		buf = buf[:n]
	}
	fileType := http.DetectContentType(buf)

	if len(setting.UploadAllowedTypes) > 0 {
		allowedTypes := strings.Split(setting.UploadAllowedTypes, ",")
		allowed := false
		for _, t := range allowedTypes {
			t := strings.Trim(t, " ")
			if t == "*/*" || t == fileType {
				allowed = true
				break
			}
		}

		if !allowed {
			ctx.Error(400, ErrFileTypeForbidden.Error())
			return
		}
	}

	up, err := models.NewUpload(header.Filename, buf, file, ctx.User.Id, ctx.Repo.Repository.ID)
	if err != nil {
		ctx.Error(500, fmt.Sprintf("NewUpload: %v", err))
		return
	}

	log.Trace("New file uploaded: %s", up.UUID)
	ctx.JSON(200, map[string]string{
		"uuid": up.UUID,
	})
}

func UploadRemoveFileFromServer(ctx *context.Context, form auth.UploadRemoveFileForm) {
	if !setting.UploadEnabled {
		ctx.Error(404, "upload is not enabled")
		return
	}

	if len(form.File) == 0 {
		ctx.Error(404, "invalid params")
		return
	}

	uuid := form.File

	if err := models.RemoveUpload(uuid, ctx.User.Id, ctx.Repo.Repository.ID); err != nil {
		ctx.Error(500, fmt.Sprintf("RemoveUpload: %v", err))
		return
	}

	log.Trace("Upload file removed: %s", uuid)
	ctx.JSON(200, map[string]string{
		"uuid": uuid,
	})
}