// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package nett

import (
	"net"
	"sync"
)

type streamListener struct {
	net, addr string
	ln        net.Listener
}

type dualStackServer struct {
	lnmu sync.RWMutex
	lns  []streamListener
	port string

	cmu sync.RWMutex
	cs  []net.Conn // established connections at the passive open side
}

func (dss *dualStackServer) buildup(server func(*dualStackServer, net.Listener)) error {
	for i := range dss.lns {
		go server(dss, dss.lns[i].ln)
	}
	return nil
}

func (dss *dualStackServer) putConn(c net.Conn) error {
	dss.cmu.Lock()
	dss.cs = append(dss.cs, c)
	dss.cmu.Unlock()
	return nil
}

func (dss *dualStackServer) teardownNetwork(net string) error {
	dss.lnmu.Lock()
	for i := range dss.lns {
		if net == dss.lns[i].net && dss.lns[i].ln != nil {
			dss.lns[i].ln.Close()
			dss.lns[i].ln = nil
		}
	}
	dss.lnmu.Unlock()
	return nil
}

func (dss *dualStackServer) teardown() error {
	dss.lnmu.Lock()
	for i := range dss.lns {
		if dss.lns[i].ln != nil {
			dss.lns[i].ln.Close()
		}
	}
	dss.lnmu.Unlock()
	dss.cmu.Lock()
	for _, c := range dss.cs {
		c.Close()
	}
	dss.cmu.Unlock()
	return nil
}

func newDualStackServer(lns []streamListener) (*dualStackServer, error) {
	dss := &dualStackServer{lns: lns, port: "0"}
	for i := range dss.lns {
		ln, err := net.Listen(dss.lns[i].net, dss.lns[i].addr+":"+dss.port)
		if err != nil {
			dss.teardown()
			return nil, err
		}
		dss.lns[i].ln = ln
		if dss.port == "0" {
			if _, dss.port, err = net.SplitHostPort(ln.Addr().String()); err != nil {
				dss.teardown()
				return nil, err
			}
		}
	}
	return dss, nil
}
