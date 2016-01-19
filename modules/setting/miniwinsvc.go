// +build miniwinsvc

// Copyright 2016 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package setting

import (
	_ "github.com/kardianos/minwinsvc"
)

func init() {
	SupportMiniWinService = true
}
