// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"path/filepath"
	"strings"
)

func processSysrootFlag(builder *commandBuilder) string {
	fromUser := false
	for _, arg := range builder.args {
		if arg.fromUser && strings.HasPrefix(arg.value, "--sysroot=") {
			fromUser = true
			break
		}
	}
	sysroot := builder.env.getenv("SYSROOT")
	if sysroot != "" {
		builder.updateEnv("SYSROOT=")
	} else {
		// Use the bundled sysroot by default.
		sysroot = filepath.Join(builder.rootPath, "usr", builder.target.target)
	}
	if !fromUser {
		builder.addPreUserArgs("--sysroot=" + sysroot)
	}
	return sysroot
}
