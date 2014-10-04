// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build !go1.3

package nett

import (
	"net"
	"time"
)

func (d *Dialer) netDialer(deadline time.Time) net.Dialer {
	return net.Dialer{
		Deadline:  deadline,
		LocalAddr: d.LocalAddr,
	}
}
