package app

type Server struct {
}

func NewServer() *Server {
	return &Server{}
}

func (s *Server) Run(stop <-chan struct{}) {
	// Your application logic here

	// start router
	startRouter(stop)
}
