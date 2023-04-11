// Copyright 2021 - 2023 Matrix Origin
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package proxy

import (
	"context"

	"github.com/fagongzi/goetty/v2"
	"github.com/matrixorigin/matrixone/pkg/common/runtime"
	"github.com/matrixorigin/matrixone/pkg/common/stopper"
	"github.com/matrixorigin/matrixone/pkg/frontend"
)

type Server struct {
	runtime runtime.Runtime
	stopper *stopper.Stopper
	config  Config
	app     goetty.NetApplication

	// handler handles the client connection.
	handler *handler
}

// NewServer creates the proxy server.
//
// NB: runtime must be included in opts.
func NewServer(ctx context.Context, config Config, opts ...Option) (*Server, error) {
	config.FillDefault()
	s := &Server{
		config: config,
	}
	for _, opt := range opts {
		opt(s)
	}
	if s.runtime == nil {
		panic("runtime of proxy is not set")
	}
	s.stopper = stopper.NewStopper("mo-proxy", stopper.WithLogger(s.runtime.Logger().RawLogger()))
	h, err := newProxyHandler(ctx, s.runtime, s.config, s.stopper)
	if err != nil {
		return nil, err
	}
	s.handler = h
	app, err := goetty.NewApplication(config.ListenAddress, nil,
		goetty.WithAppLogger(s.runtime.Logger().RawLogger()),
		goetty.WithAppHandleSessionFunc(s.handler.handle),
		goetty.WithAppSessionOptions(
			goetty.WithSessionCodec(frontend.NewSqlCodec()),
			goetty.WithSessionLogger(s.runtime.Logger().RawLogger()),
		),
	)
	if err != nil {
		return nil, err
	}
	s.app = app
	return s, nil
}

// Start starts the proxy server.
func (s *Server) Start() error {
	return s.app.Start()
}

// Close closes the proxy server.
func (s *Server) Close() error {
	_ = s.handler.Close()
	return s.app.Stop()
}