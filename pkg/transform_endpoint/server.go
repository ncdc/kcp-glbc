/*
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

package transform_endpoint

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/kuadrant/kcp-glbc/pkg/_internal/log"
)

const defaultTransformEndpoint = "/transform"

type Server struct {
	httpServer http.Server
	listener   net.Listener
}

func NewServer(port int) (*Server, error) {

	if port == 0 {
		return nil, fmt.Errorf("port cannot be 0")
	}

	listener, err := net.Listen("tcp", ":"+strconv.Itoa(port))
	if err != nil {
		return nil, err
	}

	mux := http.NewServeMux()
	mux.Handle(defaultTransformEndpoint, mockHandler())

	return &Server{
		listener: listener,
		httpServer: http.Server{
			Handler: mux,
		},
	}, nil
}

func (s *Server) Start(ctx context.Context) (err error) {
	errCh := make(chan error)

	log.Logger.Info("Started serving transform endpoint", "address", s.listener.Addr())
	if e := s.httpServer.Serve(s.listener); e != http.ErrServerClosed {
		err = e
	}

	select {

	case <-ctx.Done():
		log.Logger.Info("Stopping transform server")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		return s.httpServer.Shutdown(shutdownCtx)

	case err := <-errCh:
		log.Logger.Error(err, "Server failed")
		return err
	}

}
