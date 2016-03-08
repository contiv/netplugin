/***
Copyright 2014 Cisco Systems Inc. All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package utils

import (
	"net"
	"sync"
)

// ListenWrapper is a wrapper over net.Listener
func ListenWrapper(l net.Listener) net.Listener {
	return &contivListener{
		Listener: l,
		cond:     sync.NewCond(&sync.Mutex{})}
}

type contivListener struct {
	net.Listener
	cond   *sync.Cond
	refCnt int
}

func (s *contivListener) incrementRef() {
	s.cond.L.Lock()
	s.refCnt++
	s.cond.L.Unlock()
}

func (s *contivListener) decrementRef() {
	s.cond.L.Lock()
	s.refCnt--
	newRefs := s.refCnt
	s.cond.L.Unlock()
	if newRefs == 0 {
		s.cond.Broadcast()
	}
}

// Accept is a wrapper over regular Accept call
// which also maintains the refCnt
func (s *contivListener) Accept() (net.Conn, error) {
	s.incrementRef()
	defer s.decrementRef()
	return s.Listener.Accept()
}

// Close closes the contivListener.
func (s *contivListener) Close() error {
	if err := s.Listener.Close(); err != nil {
		return err
	}

	s.cond.L.Lock()
	for s.refCnt > 0 {
		s.cond.Wait()
	}
	s.cond.L.Unlock()
	return nil
}
