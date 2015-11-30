// Copyright 2014 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package v1

import (
	api "github.com/kiliit/go-gogs-client"

	"github.com/gogits/gogs/models"
	"github.com/gogits/gogs/modules/git"
	"github.com/gogits/gogs/modules/log"
	"github.com/gogits/gogs/modules/middleware"
	"github.com/gogits/gogs/modules/template"
)

func ToApiSignature(signature *git.Signature) *api.Signature {
	return &api.Signature{
		Email:           signature.Email,
		Name:            signature.Name,
		When:            signature.When,
	}
}

func ToApiCommit(commit *git.Commit) *api.Commit {
	return &api.Commit{
		ID:                commit.ID.String(),
		Author:            *ToApiSignature(commit.Author),
		Committer:         *ToApiSignature(commit.Committer),
		CommitMessage:     commit.CommitMessage,
	}
}

func HEADCommit(ctx *middleware.Context) {
	ctx.JSON(200, &api.Sha1{
		Sha1: ctx.Repo.Commit.ID.String(),
	})
}

func CommitByID(ctx *middleware.Context) {
	commit, err := ctx.Repo.GitRepo.GetCommit(ctx.Params(":commitid"))
	if err != nil {
		log.Error(4, "GetCommit: %v", err)
		ctx.APIError(500, "GetCommit", err.Error())
		return
	}

	ctx.JSON(200, ToApiCommit(commit))
}

func ListCommits(ctx *middleware.Context) {
	commits, err := ctx.Repo.Commit.CommitsBefore()
	if err != nil {
		log.Error(4, "CommitsBefore: %v", err)
		ctx.APIError(500, "CommitsBefore", err)
		return
	}

	apiCommits := make([]*api.Commit, commits.Len())
	i := 0
	for e := commits.Front(); e != nil; e = e.Next() {
		apiCommits[i] = ToApiCommit(e.Value.(*git.Commit))
		i = i+ 1
	}

	ctx.JSON(200, apiCommits)
}


/**
 *******************
 *	DIFF
 *******************
 */

func ToApiDiff(diff *models.Diff) *api.Diff {
	return &api.Diff{
		TotalAddition:        diff.TotalAddition,
		TotalDeletion:        diff.TotalDeletion,
		Files:                ToApiDiffFiles(diff.Files),
	}
}

func ToApiDiffFiles(diffFiles []*models.DiffFile) []*api.DiffFile {
	apiDiffFiles := make([]*api.DiffFile, len(diffFiles))
	for i := range apiDiffFiles {
		apiDiffFiles[i] = ToApiDiffFile(diffFiles[i])
	}
	return apiDiffFiles
}

func ToApiDiffFile(diffFile *models.DiffFile) *api.DiffFile {
	return &api.DiffFile{
		Name:               diffFile.Name,
		Index:              diffFile.Index,
		Addition:           diffFile.Addition,
		Deletion:           diffFile.Deletion,
		Type:               template.DiffTypeToStr(diffFile.Type),
		IsCreated:          diffFile.IsCreated,
		IsDeleted:          diffFile.IsDeleted,
		IsBin:              diffFile.IsBin,
		Sections:           ToApiDiffSections(diffFile.Sections),
	}
}

func ToApiDiffSections(diffSections []*models.DiffSection) []*api.DiffSection {
	apiDiffSections := make([]*api.DiffSection, len(diffSections))
	for i := range apiDiffSections {
		apiDiffSections[i] = ToApiDiffSection(diffSections[i])
	}
	return apiDiffSections
}

func ToApiDiffSection(diffSection *models.DiffSection) *api.DiffSection {
	return &api.DiffSection{
		Name:	diffSection.Name,
		Lines:	ToApiDiffLines(diffSection.Lines),
	}
}

func ToApiDiffLines(diffLines []*models.DiffLine) []*api.DiffLine {
	apiDiffLines := make([]*api.DiffLine, len(diffLines))
	for i := range apiDiffLines {
		apiDiffLines[i] = ToApiDiffLine(diffLines[i])
	}
	return apiDiffLines
}

func ToApiDiffLine(diffLine *models.DiffLine) *api.DiffLine {
	return &api.DiffLine{
		LeftIdx:  diffLine.LeftIdx,
		RightIdx: diffLine.RightIdx,
		Type:     template.DiffLineTypeToStr(diffLine.Type),
		Content:  diffLine.Content,
	}
}

func DiffRange(ctx *middleware.Context) {
	beforeCommitID := ctx.Params(":before")
	afterCommitID := ctx.Params(":after")

	_, err := ctx.Repo.GitRepo.GetCommit(beforeCommitID)
	if err != nil {
		ctx.APIError(404, "GetCommit", err)
		return
	}
	_, err = ctx.Repo.GitRepo.GetCommit(afterCommitID)
	if err != nil {
		ctx.APIError(404, "GetCommit", err)
		return
	}

	diff, err := models.GetDiffRange(ctx.Repo.GitRepo.Path, beforeCommitID, afterCommitID, 10000)
	if err != nil {
		log.Error(4, "GetDiffRange: %v", err)
		ctx.APIError(500, "GetDiffRange", err.Error())
		return
	}

	ctx.JSON(200, ToApiDiff(diff))
}