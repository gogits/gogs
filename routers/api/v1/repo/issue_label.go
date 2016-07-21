// Copyright 2016 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	api "github.com/gogits/go-gogs-client"

	"github.com/gogits/gogs/models"
	"github.com/gogits/gogs/modules/context"
	"github.com/gogits/gogs/routers/api/v1/convert"
)

// Get, Add, Replace, Clear

func GetIssueLabels(ctx *context.APIContext) {
	issue, err := models.GetIssueByIndex(ctx.Repo.Repository.ID, ctx.ParamsInt64(":index"))
	if err != nil {
		if models.IsErrIssueNotExist(err) {
			ctx.Status(404)
		} else {
			ctx.Error(500, "GetIssueByIndex", err)
		}
		return
	}

	apiLabels := make([]*api.Label, len(issue.Labels))
	for i := range issue.Labels {
		apiLabels[i] = convert.ToLabel(issue.Labels[i])
	}

	ctx.JSON(200, &apiLabels)
}

func AddIssueLabels(ctx *context.APIContext, form api.IssueLabelsOption) {
	if !ctx.Repo.IsWriter() {
		ctx.Status(403)
		return
	}

	issue, err := models.GetIssueByIndex(ctx.Repo.Repository.ID, ctx.ParamsInt64(":index"))
	if err != nil {
		if models.IsErrIssueNotExist(err) {
			ctx.Status(404)
		} else {
			ctx.Error(500, "GetIssueByIndex", err)
		}
		return
	}

	var labels []*models.Label
	if labels, err = loadLabelsByID(form.Labels, issue.RepoID); err != nil {
		ctx.Error(400, "loadLabelsByID", err)
		return
	}

	for i := range labels {
		if !models.HasIssueLabel(issue.ID, labels[i].ID) {
			if err := models.NewIssueLabel(issue, labels[i]); err != nil {
				ctx.Error(500, "NewIssueLabel", err)
				return
			}
		}
	}

	updatedIssue, err := models.GetIssueByIndex(ctx.Repo.Repository.ID, ctx.ParamsInt64(":index"))
	if err != nil {
		if models.IsErrIssueNotExist(err) {
			ctx.Status(404)
		} else {
			ctx.Error(500, "GetUpdatedIssueByIndex", err)
		}
		return
	}

	apiLabels := make([]*api.Label, len(updatedIssue.Labels))
	for i := range updatedIssue.Labels {
		apiLabels[i] = convert.ToLabel(updatedIssue.Labels[i])
	}

	ctx.JSON(200, &apiLabels)
}

func ReplaceIssueLabels(ctx *context.APIContext, form api.IssueLabelsOption) {
	if !ctx.Repo.IsWriter() {
		ctx.Status(403)
		return
	}

	issue, err := models.GetIssueByIndex(ctx.Repo.Repository.ID, ctx.ParamsInt64(":index"))
	if err != nil {
		if models.IsErrIssueNotExist(err) {
			ctx.Status(404)
		} else {
			ctx.Error(500, "GetIssueByIndex", err)
		}
		return
	}

	var labels []*models.Label
	if labels, err = loadLabelsByID(form.Labels, issue.RepoID); err != nil {
		ctx.Error(400, "loadLabelsByID", err)
		return
	}

	for i := range issue.Labels {
		if err := models.DeleteIssueLabel(issue, issue.Labels[i]); err != nil {
			ctx.Error(500, "NewIssueLabel", err)
			return
		}
	}

	for i := range labels {
		if !models.HasIssueLabel(issue.ID, labels[i].ID) {
			if err := models.NewIssueLabel(issue, labels[i]); err != nil {
				ctx.Error(500, "NewIssueLabel", err)
				return
			}
		}
	}

	updatedIssue, err := models.GetIssueByIndex(ctx.Repo.Repository.ID, ctx.ParamsInt64(":index"))
	if err != nil {
		if models.IsErrIssueNotExist(err) {
			ctx.Status(404)
		} else {
			ctx.Error(500, "GetUpdatedIssueByIndex", err)
		}
		return
	}

	apiLabels := make([]*api.Label, len(updatedIssue.Labels))
	for i := range updatedIssue.Labels {
		apiLabels[i] = convert.ToLabel(updatedIssue.Labels[i])
	}

	ctx.JSON(200, &apiLabels)
}

func DeleteIssueLabel(ctx *context.APIContext) {
	if !ctx.Repo.IsWriter() {
		ctx.Status(403)
		return
	}

	issue, err := models.GetIssueByIndex(ctx.Repo.Repository.ID, ctx.ParamsInt64(":index"))
	if err != nil {
		if models.IsErrIssueNotExist(err) {
			ctx.Status(404)
		} else {
			ctx.Error(500, "GetIssueByIndex", err)
		}
		return
	}

	label, err := models.GetLabelByID(ctx.ParamsInt64(":id"))
	if err != nil {
		if models.IsErrLabelNotExist(err) {
			ctx.Status(400)
		} else {
			ctx.Error(500, "GetLabelByID", err)
		}
		return
	}

	if err := models.DeleteIssueLabel(issue, label); err != nil {
		ctx.Error(500, "DeleteIssueLabel", err)
		return
	}

	ctx.Status(204)
}

func DeleteAllIssueLabels(ctx *context.APIContext) {
	if !ctx.Repo.IsWriter() {
		ctx.Status(403)
		return
	}

	issue, err := models.GetIssueByIndex(ctx.Repo.Repository.ID, ctx.ParamsInt64(":index"))
	if err != nil {
		if models.IsErrIssueNotExist(err) {
			ctx.Status(404)
		} else {
			ctx.Error(500, "GetIssueByIndex", err)
		}
		return
	}

	for i := range issue.Labels {
		if err := models.DeleteIssueLabel(issue, issue.Labels[i]); err != nil {
			ctx.Error(500, "DeleteIssueLabel", err)
			return
		}
	}

	ctx.Status(204)
}

func loadLabelsByID(labelIDs []int64, repoID int64) ([]*models.Label, error) {
	labels := make([]*models.Label, 0, len(labelIDs))
	errors := make([]error, 0, len(labelIDs))

	for i := range labelIDs {
		label, err := models.GetLabelByID(labelIDs[i])
		if err != nil {
			errors = append(errors, err)
		} else if label.RepoID != repoID {
			errors = append(errors, models.ErrLabelNotValidForRepository{label.ID, repoID})
		} else {
			labels = append(labels, label)
		}
	}

	errorCount := len(errors)

	if errorCount == 1 {
		return labels, errors[0]
	} else if errorCount > 1 {
		return labels, models.ErrMultipleErrors{errors}
	}

	return labels, nil
}
