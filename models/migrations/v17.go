// Copyright 2017 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"strings"

	"github.com/go-xorm/xorm"
)

func removeInvalidProtectBranchWhitelist(x *xorm.Engine) error {
	_, err := x.Exec("DELETE FROM protect_branch_whitelist WHERE protect_branch_id = 0")
	if err != nil && (strings.Contains(err.Error(), "no such table") || strings.Contains(err.Error(), "doesn't exist")) {
		return nil
	}
	return err
}
