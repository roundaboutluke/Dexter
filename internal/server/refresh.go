package server

func refreshAlertState(s *Server) {
	if s == nil || s.processor == nil {
		return
	}
	s.processor.RefreshAlertCacheAsync()
}

func refreshAlertStateSync(s *Server) error {
	if s == nil || s.processor == nil {
		return nil
	}
	return s.processor.RefreshAlertCacheSync()
}
